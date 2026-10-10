package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-ai-study/api/analysis"
	"sync"
	"time"

	"go-ai-study/api/fault"
	"go-ai-study/api/jobs"
	"go-ai-study/api/models"
	"go-ai-study/api/sourcefiles"
	"go-ai-study/internal/source"
)

type Repository interface {
	jobs.Store
	OwnedProject(context.Context, uint, uint) (models.Project, error)
	OwnedJob(context.Context, string, uint) (models.Job, error)
}

type Source struct {
	repository Repository
	workspace  *sourcefiles.Workspace
	engine     *jobs.Engine
	rag        *ragOperations
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	stopping   bool
	operations sync.WaitGroup
}

func (s *Source) Ready(ctx context.Context) error {
	s.mu.Lock()
	stopping := s.stopping || s.ctx.Err() != nil
	s.mu.Unlock()
	if stopping {
		return fault.ErrStopping
	}
	if ready, ok := s.repository.(interface{ Ready(context.Context) error }); ok {
		return ready.Ready(ctx)
	}
	return fault.ErrStorage
}

func NewSource(ctx context.Context, repository Repository, root string, workers, queueSize int, ragOptions ...RAGSettings) (*Source, error) {
	if ctx == nil || repository == nil {
		return nil, fmt.Errorf("source context and repository are required")
	}
	var bindings []sourcefiles.Binding
	var localRoot *sourcefiles.LocalRoot
	if len(ragOptions) > 0 {
		bindings = ragOptions[0].SourceBindings
		localRoot = ragOptions[0].LocalRoot
	}
	workspace, err := sourcefiles.NewWithConfig(root, bindings, localRoot)
	if err != nil {
		return nil, err
	}
	serviceCtx, cancel := context.WithCancel(ctx)
	s := &Source{repository: repository, workspace: workspace, ctx: serviceCtx, cancel: cancel}
	s.rag = newRAGOperations(ragOptions...)
	timeout := 15 * time.Minute
	if len(ragOptions) > 0 && ragOptions[0].JobTimeout > 0 {
		timeout = ragOptions[0].JobTimeout
	}
	s.engine, err = jobs.NewTimed(serviceCtx, repository, s.execute, workers, queueSize, timeout, func(job models.Job) {
		if job.Kind == models.JobIndex {
			s.rag.release(job.ProjectID)
		}
	})
	if err != nil {
		cancel()
		return nil, err
	}
	return s, nil
}

func (s *Source) Scan(ctx context.Context, ownerID, projectID uint, target string) (source.ScanResult, error) {
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return source.ScanResult{}, err
	}
	defer done()
	if _, err := s.repository.OwnedProject(ctx, projectID, ownerID); err != nil {
		return source.ScanResult{}, err
	}
	scope, err := s.workspace.Resolve(ownerID, projectID, target)
	if err != nil {
		return source.ScanResult{}, err
	}
	return scope.Scan(ctx)
}

func (s *Source) SubmitAnalyze(ctx context.Context, ownerID, projectID uint, target string) (models.Job, error) {
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return models.Job{}, err
	}
	defer done()
	if _, err := s.repository.OwnedProject(ctx, projectID, ownerID); err != nil {
		return models.Job{}, err
	}
	scope, err := s.workspace.Resolve(ownerID, projectID, target)
	if err != nil {
		return models.Job{}, err
	}
	// Missing/unreadable sources are durable failed work. Unsafe existing
	// bindings are rejected immediately and checked again by the worker.
	if err := scope.Check(); errors.Is(err, fault.ErrUnsafePath) || errors.Is(err, fault.ErrInvalidTarget) || errors.Is(err, fault.ErrExcludedTarget) {
		return models.Job{}, err
	}
	return s.engine.SubmitWithExecutor(ctx, models.Job{OwnerID: ownerID, ProjectID: projectID, Kind: models.JobAnalyze, Target: scope.Target()}, s.scopeExecutor(scope))
}

func (s *Source) GetJob(ctx context.Context, ownerID uint, id string) (models.Job, error) {
	return s.repository.OwnedJob(ctx, id, ownerID)
}

func (s *Source) SourceInfo(ctx context.Context, ownerID, projectID uint) (sourcefiles.SourceInfo, error) {
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return sourcefiles.SourceInfo{}, err
	}
	defer done()
	if _, err := s.repository.OwnedProject(ctx, projectID, ownerID); err != nil {
		return sourcefiles.SourceInfo{}, err
	}
	return s.workspace.Info(ownerID, projectID), nil
}

func (s *Source) execute(ctx context.Context, job models.Job) (json.RawMessage, error) {
	if _, err := s.repository.OwnedProject(ctx, job.ProjectID, job.OwnerID); err != nil {
		return nil, err
	}
	if job.Kind == models.JobIndex {
		return s.index(ctx, job)
	}
	if job.Kind == models.JobLegacyAnalyze {
		result, err := analysis.PerformAnalysisContext(ctx, job.Payload)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(result.ToJSON()), nil
	}
	if job.Kind != models.JobAnalyze {
		return nil, fault.ErrInvalidSource
	}
	result, err := s.workspace.Analyze(ctx, job.OwnerID, job.ProjectID, job.Target)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

// The captured scope preserves the selected root after job.Target becomes relative.
func (s *Source) scopeExecutor(scope sourcefiles.Scope) jobs.Executor {
	return func(ctx context.Context, job models.Job) (json.RawMessage, error) {
		if _, err := s.repository.OwnedProject(ctx, job.ProjectID, job.OwnerID); err != nil {
			return nil, err
		}
		if job.Kind == models.JobIndex {
			return s.indexScope(ctx, job, scope)
		}
		result, err := scope.Analyze(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	}
}

func (s *Source) begin(ctx context.Context) (context.Context, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopping || s.ctx.Err() != nil {
		return nil, nil, fault.ErrStopping
	}
	s.operations.Add(1)
	operationCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	return operationCtx, func() { stop(); cancel(); s.operations.Done() }, nil
}
func (s *Source) Stop() {
	s.mu.Lock()
	s.stopping = true
	s.cancel()
	s.mu.Unlock()
	go s.engine.Stop()
}
func (s *Source) Shutdown(ctx context.Context) error {
	s.Stop()
	done := make(chan struct{})
	go func() { s.operations.Wait(); close(done) }()
	if err := s.engine.Shutdown(ctx); err != nil {
		return err
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (s *Source) CancelJob(ctx context.Context, owner uint, id string) (models.Job, error) {
	if _, err := s.repository.OwnedJob(ctx, id, owner); err != nil {
		return models.Job{}, err
	}
	if err := s.engine.Cancel(id); err != nil {
		return models.Job{}, err
	}
	return s.repository.OwnedJob(ctx, id, owner)
}
func (s *Source) SubmitLegacy(ctx context.Context, owner, project uint, code string) (models.Job, error) {
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return models.Job{}, err
	}
	defer done()
	if _, err := s.repository.OwnedProject(ctx, project, owner); err != nil {
		return models.Job{}, err
	}
	return s.engine.SubmitJob(ctx, models.Job{OwnerID: owner, ProjectID: project, Kind: models.JobLegacyAnalyze, Target: ".", Payload: code})
}
