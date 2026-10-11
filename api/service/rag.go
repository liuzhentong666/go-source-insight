package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go-ai-study/api/fault"
	"go-ai-study/api/models"
	"go-ai-study/api/sourcefiles"
	"go-ai-study/internal/ai"
	coreconfig "go-ai-study/internal/config"
	"go-ai-study/internal/localfiles"
	"go-ai-study/internal/source"
)

type RAGClient interface {
	Index(context.Context, string) (*ai.IndexSummary, error)
	Ask(context.Context, string, string) (*ai.AnswerResult, error)
	Close() error
}
type RAGFactory func(context.Context, ai.FactoryOptions) (RAGClient, error)
type RAGSettings struct {
	SourceAccessMode string
	SourceReader     localfiles.Provider
	SourceBindings   []sourcefiles.Binding
	LocalRoot        *sourcefiles.LocalRoot
	Factory          RAGFactory
	LoadConfig       func() (*coreconfig.Config, error)
	JobTimeout       time.Duration
}
type ragOperations struct {
	factory    RAGFactory
	loadConfig func() (*coreconfig.Config, error)
	mu         sync.Mutex
	busy       map[uint]bool
}

func newRAGOperations(settings ...RAGSettings) *ragOperations {
	r := &ragOperations{busy: map[uint]bool{}, factory: func(ctx context.Context, opts ai.FactoryOptions) (RAGClient, error) {
		return ai.NewOllamaRAGService(ctx, opts)
	}, loadConfig: func() (*coreconfig.Config, error) { return coreconfig.LoadUnvalidated("") }}
	if len(settings) > 0 {
		if settings[0].Factory != nil {
			r.factory = settings[0].Factory
		}
		if settings[0].LoadConfig != nil {
			r.loadConfig = settings[0].LoadConfig
		}
	}
	return r
}

func ProjectCollection(projectID uint) string { return fmt.Sprintf("api_project_%d", projectID) }
func (r *ragOperations) reserve(project uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.busy[project] {
		return fault.ErrProjectBusy
	}
	r.busy[project] = true
	return nil
}
func (r *ragOperations) release(project uint) { r.mu.Lock(); delete(r.busy, project); r.mu.Unlock() }

func (s *Source) SubmitIndex(ctx context.Context, ownerID, projectID uint, target string) (models.Job, error) {
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return models.Job{}, err
	}
	defer done()
	if _, err := s.repository.OwnedProject(ctx, projectID, ownerID); err != nil {
		return models.Job{}, err
	}
	scope, err := s.workspace.ResolveContext(ctx, ownerID, projectID, target)
	if err != nil {
		return models.Job{}, err
	}
	if err := scope.CheckContext(ctx); errors.Is(err, fault.ErrUnsafePath) || errors.Is(err, fault.ErrInvalidTarget) || errors.Is(err, fault.ErrExcludedTarget) {
		return models.Job{}, err
	}
	if err := s.rag.reserve(projectID); err != nil {
		return models.Job{}, err
	}
	job, err := s.engine.SubmitWithExecutor(ctx, models.Job{OwnerID: ownerID, ProjectID: projectID, Kind: models.JobIndex, Target: scope.Target()}, s.scopeExecutor(scope))
	if err != nil {
		s.rag.release(projectID)
	}
	return job, err
}

func (s *Source) index(ctx context.Context, job models.Job) (json.RawMessage, error) {
	scope, err := s.workspace.ResolveContext(ctx, job.OwnerID, job.ProjectID, job.Target)
	if err != nil {
		return nil, err
	}
	return s.indexScope(ctx, job, scope)
}

func (s *Source) indexScope(ctx context.Context, job models.Job, scope sourcefiles.Scope) (json.RawMessage, error) {
	var output json.RawMessage
	err := scope.WithFSContext(ctx, func(files fs.FS) error {
		return s.rag.run(ctx, job.ProjectID, &ai.FSScanner{FS: files}, true, func(ctx context.Context, client RAGClient) error {
			summary, err := client.Index(ctx, job.Target)
			if err != nil {
				return ragError(ctx, err, fault.ErrIndexFailed)
			}
			if summary == nil || summary.Collection != ProjectCollection(job.ProjectID) || summary.TargetPath != job.Target || summary.FilesScanned < 0 || summary.ChunksCreated < 0 || summary.ChunksIndexed != summary.ChunksCreated {
				return fault.ErrIndexFailed
			}
			output, err = json.Marshal(summary)
			return err
		})
	})
	return output, err
}

func (s *Source) Ask(ctx context.Context, ownerID, projectID uint, question, file string) (*ai.AnswerResult, error) {
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	if _, err := s.repository.OwnedProject(ctx, projectID, ownerID); err != nil {
		return nil, err
	}
	question = strings.TrimSpace(question)
	if question == "" || utf8.RuneCountInString(question) > 4096 || len(question) > 16<<10 {
		return nil, fault.ErrInvalidQuestion
	}
	if file != "" {
		normalized, err := sourcefiles.NormalizeTarget(file)
		if err != nil || normalized == "." {
			return nil, fault.ErrInvalidTarget
		}
		file = normalized
	}
	if err := s.rag.reserve(projectID); err != nil {
		return nil, err
	}
	defer s.rag.release(projectID)
	var answer *ai.AnswerResult
	err = s.rag.run(ctx, projectID, nil, false, func(ctx context.Context, client RAGClient) error {
		result, err := client.Ask(ctx, question, file)
		if err != nil {
			return ragError(ctx, err, fault.ErrAskFailed)
		}
		if result == nil {
			return fault.ErrAskFailed
		}
		for _, citation := range result.Sources {
			name, err := sourcefiles.NormalizeTarget(citation.Source)
			if err != nil || citation.Source == "" || name == "." || citation.StartLine < 1 || citation.EndLine < citation.StartLine {
				return fault.ErrAskFailed
			}
		}
		if result.Sources == nil {
			result.Sources = []ai.SearchResult{}
		}
		answer = result
		return nil
	})
	if err != nil {
		return nil, err
	}
	return answer, nil
}

// Configuration and clients are created only inside a model operation. The
// operation context spans factory checks, retrieval/model calls, and closing.
func (r *ragOperations) run(ctx context.Context, project uint, scanner ai.CodeScanner, publication bool, operation func(context.Context, RAGClient) error) (err error) {
	if err := ctx.Err(); err != nil {
		return ragError(ctx, err, fault.ErrAIUnavailable)
	}
	cfg, err := r.loadConfig()
	if err != nil || cfg == nil {
		return fault.ErrAIUnavailable
	}
	copyConfig := *cfg
	copyConfig.RAGConfig.Collection = ProjectCollection(project)
	if err := copyConfig.ValidateRAG(); err != nil {
		return fault.ErrAIUnavailable
	}
	config := copyConfig.RAGConfig
	opCtx, cancel := context.WithTimeout(ctx, time.Duration(copyConfig.RequestTimeoutSeconds)*time.Second)
	defer cancel()
	client, err := r.factory(opCtx, ai.FactoryOptions{
		OllamaEndpoint: copyConfig.OllamaEndpoint, MilvusEndpoint: copyConfig.MilvusEndpoint, Timeout: time.Duration(copyConfig.RequestTimeoutSeconds) * time.Second, Scanner: scanner,
		RAG: ai.RAGOptions{Collection: config.Collection, EmbeddingModel: config.EmbeddingModel, ChatModel: config.ChatModel, VectorDim: config.VectorDim, TopK: config.TopK, ChunkMaxLines: config.ChunkMaxLines, ChunkMinLines: config.ChunkMinLines},
	})
	if client != nil {
		defer func() {
			if closeErr := client.Close(); closeErr != nil {
				log.Printf("RAG connection close failed project=%d", project)
				if err == nil && !publication {
					err = fault.ErrAIUnavailable
				}
			}
		}()
	}
	if err != nil {
		return ragError(opCtx, err, fault.ErrAIUnavailable)
	}
	if client == nil {
		return fault.ErrAIUnavailable
	}
	err = operation(opCtx, client)
	if err == nil && opCtx.Err() != nil && !publication {
		return ragError(opCtx, opCtx.Err(), fault.ErrAIUnavailable)
	}
	return err
}

func ragError(ctx context.Context, err, fallback error) error {
	if errors.Is(err, ai.ErrPublicationUnconfirmed) {
		return fault.ErrPublicationUnknown
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fault.ErrTimeout
	}
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return fault.ErrCanceled
	}
	if errors.Is(err, fault.ErrUnsafePath) {
		return fault.ErrUnsafePath
	}
	var accessError *source.AccessError
	if errors.As(err, &accessError) {
		return fault.ErrSourceUnavailable
	}
	return fallback
}
