package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"go-ai-study/api/fault"
	"go-ai-study/api/models"
)

type testStore struct {
	mu        sync.Mutex
	rows      map[string]models.Job
	events    chan models.Job
	createErr error
}

func (s *testStore) CreateJob(_ context.Context, j *models.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return s.createErr
	}
	s.rows[j.ID] = *j
	s.events <- *j
	return nil
}
func (s *testStore) UpdateJob(_ context.Context, id, expected string, changes map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.rows[id]
	if !ok || j.Status != expected {
		return fault.ErrConflict
	}
	j.Status = changes["status"].(string)
	if value, ok := changes["result"].(string); ok {
		j.Result = json.RawMessage(value)
	}
	if value, ok := changes["failure_code"].(string); ok {
		j.FailureCode = value
	}
	if value, ok := changes["failure_reason"].(string); ok {
		j.FailureReason = value
	}
	s.rows[id] = j
	s.events <- j
	return nil
}
func newStore() *testStore {
	return &testStore{rows: map[string]models.Job{}, events: make(chan models.Job, 64)}
}
func awaitStatus(t *testing.T, s *testStore, id, status string) models.Job {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case j := <-s.events:
			if j.ID == id && j.Status == status {
				return j
			}
		case <-timer.C:
			t.Fatalf("job %s never reached %s", id, status)
			return models.Job{}
		}
	}
}

func TestEnginePersistsStatesAndBoundsOutstandingJobs(t *testing.T) {
	store := newStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	e, err := New(ctx, store, func(context.Context, models.Job) (json.RawMessage, error) {
		started <- struct{}{}
		<-release
		return json.RawMessage(`{"files_analyzed":1}`), nil
	}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	first, err := e.Submit(context.Background(), 7, 1, models.JobAnalyze, ".")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start")
	}
	awaitStatus(t, store, first.ID, models.JobRunning)
	second, err := e.Submit(context.Background(), 7, 1, models.JobAnalyze, "pkg")
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	queued := store.rows[second.ID].Status
	count := len(store.rows)
	store.mu.Unlock()
	if queued != models.JobQueued || count != 2 {
		t.Fatalf("waiting job was not durable: status=%s count=%d", queued, count)
	}
	if _, err := e.Submit(context.Background(), 7, 1, models.JobAnalyze, "."); !errors.Is(err, fault.ErrQueueFull) {
		t.Fatalf("full queue accepted job: %v", err)
	}
	store.mu.Lock()
	count = len(store.rows)
	store.mu.Unlock()
	if count != 2 {
		t.Fatal("rejected job created an orphan row")
	}
	close(release)
	completed := awaitStatus(t, store, first.ID, models.JobCompleted)
	if !json.Valid(completed.Result) {
		t.Fatal("completed result not persisted")
	}
	awaitStatus(t, store, second.ID, models.JobCompleted)
}

func TestEngineFailuresAreDurableAndDoNotLeakDetails(t *testing.T) {
	for name, executor := range map[string]Executor{
		"error": func(context.Context, models.Job) (json.RawMessage, error) {
			return nil, errors.New("private filesystem or database detail")
		},
		"panic":          func(context.Context, models.Job) (json.RawMessage, error) { panic("private runtime detail") },
		"invalid result": func(context.Context, models.Job) (json.RawMessage, error) { return json.RawMessage("broken"), nil },
	} {
		t.Run(name, func(t *testing.T) {
			store := newStore()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			e, err := New(ctx, store, executor, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			j, err := e.Submit(context.Background(), 7, 1, models.JobAnalyze, ".")
			if err != nil {
				t.Fatal(err)
			}
			failed := awaitStatus(t, store, j.ID, models.JobFailed)
			if failed.FailureCode != "operation_failed" || failed.FailureReason != "operation failed" || failed.Result != nil {
				t.Fatalf("unsafe failure: %+v", failed)
			}
		})
	}
}

func TestEngineCreateFailureReleasesCapacity(t *testing.T) {
	store := newStore()
	store.createErr = fault.ErrStorage
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e, err := New(ctx, store, func(context.Context, models.Job) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := e.Submit(context.Background(), 7, 1, models.JobAnalyze, "."); !errors.Is(err, fault.ErrStorage) {
			t.Fatalf("slot leaked after create failure: %v", err)
		}
	}
	store.mu.Lock()
	store.createErr = nil
	store.mu.Unlock()
	j, err := e.Submit(context.Background(), 7, 1, models.JobAnalyze, ".")
	if err != nil {
		t.Fatal(err)
	}
	awaitStatus(t, store, j.ID, models.JobCompleted)
}
