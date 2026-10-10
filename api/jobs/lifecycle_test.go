package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"go-ai-study/api/fault"
	"go-ai-study/api/models"
	"sync/atomic"
	"testing"
	"time"
)

func TestCancelQueuedFreesCapacityAndRunningCancellation(t *testing.T) {
	store := newStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan string, 3)
	released := make(chan string, 3)
	e, err := New(ctx, store, func(ctx context.Context, j models.Job) (json.RawMessage, error) {
		started <- j.ID
		<-ctx.Done()
		return nil, ctx.Err()
	}, 1, 1, func(j models.Job) { released <- j.ID })
	if err != nil {
		t.Fatal(err)
	}
	first, _ := e.Submit(ctx, 7, 1, models.JobIndex, ".")
	<-started
	second, _ := e.Submit(ctx, 7, 2, models.JobIndex, ".")
	if err = e.Cancel(second.ID); err != nil {
		t.Fatal(err)
	}
	awaitStatus(t, store, second.ID, models.JobCanceled)
	if got := <-released; got != second.ID {
		t.Fatalf("queued callback=%s", got)
	}
	third, err := e.Submit(ctx, 7, 2, models.JobIndex, ".")
	if err != nil {
		t.Fatalf("capacity not released: %v", err)
	}
	if err = e.Cancel(first.ID); err != nil {
		t.Fatal(err)
	}
	awaitStatus(t, store, first.ID, models.JobCanceling)
	awaitStatus(t, store, first.ID, models.JobCanceled)
	if got := <-started; got != third.ID {
		t.Fatalf("canceled queued job executed: %s", got)
	}
	if err = e.Cancel(first.ID); err != nil {
		t.Fatal(err)
	}
	shutdownCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err = e.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	if _, err = e.Submit(ctx, 7, 2, models.JobAnalyze, "."); !errors.Is(err, fault.ErrStopping) {
		t.Fatalf("shutdown admission=%v", err)
	}
}

func TestSuccessfulPublicationWinsCancellation(t *testing.T) {
	store := newStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	committed := make(chan struct{})
	release := make(chan struct{})
	e, _ := New(ctx, store, func(ctx context.Context, j models.Job) (json.RawMessage, error) {
		close(committed)
		<-release
		return json.RawMessage(`{"published":true}`), nil
	}, 1, 1)
	j, _ := e.Submit(ctx, 7, 1, models.JobIndex, ".")
	<-committed
	if err := e.Cancel(j.ID); err != nil {
		t.Fatal(err)
	}
	awaitStatus(t, store, j.ID, models.JobCanceling)
	close(release)
	result := awaitStatus(t, store, j.ID, models.JobCompleted)
	if string(result.Result) != `{"published":true}` {
		t.Fatalf("lost committed result: %+v", result)
	}
	if err := e.Cancel(j.ID); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	status := store.rows[j.ID].Status
	store.mu.Unlock()
	if status != models.JobCompleted {
		t.Fatal("terminal result overwritten")
	}
}

type failingStateStore struct {
	*testStore
	fail string
}

func (s *failingStateStore) UpdateJob(ctx context.Context, id, expected string, changes map[string]any) error {
	if changes["status"] == s.fail {
		return fault.ErrStorage
	}
	return s.testStore.UpdateJob(ctx, id, expected, changes)
}
func TestPersistenceFailureNeverInventsCompletion(t *testing.T) {
	for _, stage := range []string{models.JobRunning, models.JobCompleted} {
		t.Run(stage, func(t *testing.T) {
			store := &failingStateStore{testStore: newStore(), fail: stage}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var calls atomic.Int32
			released := make(chan struct{}, 1)
			e, _ := New(ctx, store, func(context.Context, models.Job) (json.RawMessage, error) {
				calls.Add(1)
				return json.RawMessage(`{}`), nil
			}, 1, 1, func(models.Job) { released <- struct{}{} })
			j, _ := e.Submit(ctx, 7, 1, models.JobIndex, ".")
			select {
			case <-released:
			case <-time.After(time.Second):
				t.Fatal("release callback missing")
			}
			store.mu.Lock()
			status := store.rows[j.ID].Status
			store.mu.Unlock()
			want := models.JobQueued
			wantCalls := int32(0)
			if stage == models.JobCompleted {
				want = models.JobRunning
				wantCalls = 1
			}
			if status != want || calls.Load() != wantCalls {
				t.Fatalf("false durable state=%s calls=%d", status, calls.Load())
			}
		})
	}
}
func TestExecutionTimeoutAndBoundedShutdown(t *testing.T) {
	store := newStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e, _ := NewTimed(ctx, store, func(ctx context.Context, j models.Job) (json.RawMessage, error) { <-ctx.Done(); return nil, ctx.Err() }, 1, 1, 20*time.Millisecond)
	j, _ := e.Submit(ctx, 7, 1, models.JobAnalyze, ".")
	failed := awaitStatus(t, store, j.ID, models.JobFailed)
	if failed.FailureCode != "timeout" {
		t.Fatalf("timeout=%+v", failed)
	}
	shutdownCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := e.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	// Noncooperative dependencies cannot block the caller's shutdown deadline.
	release := make(chan struct{})
	started := make(chan struct{})
	other, _ := New(ctx, newStore(), func(context.Context, models.Job) (json.RawMessage, error) {
		close(started)
		<-release
		return json.RawMessage(`{}`), nil
	}, 1, 1)
	_, _ = other.Submit(ctx, 7, 1, models.JobAnalyze, ".")
	<-started
	short, stopShort := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stopShort()
	if err := other.Shutdown(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unbounded shutdown=%v", err)
	}
	close(release)
	if err := other.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
}

func TestCancelPersistenceFailureDoesNotClaimAccepted(t *testing.T) {
	store := &failingStateStore{testStore: newStore(), fail: models.JobCanceling}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	e, _ := New(ctx, store, func(ctx context.Context, j models.Job) (json.RawMessage, error) {
		close(started)
		<-release
		return json.RawMessage(`{}`), nil
	}, 1, 1)
	j, _ := e.Submit(ctx, 7, 1, models.JobAnalyze, ".")
	<-started
	if err := e.Cancel(j.ID); !errors.Is(err, fault.ErrStorage) {
		t.Fatalf("cancel falsely accepted=%v", err)
	}
	store.mu.Lock()
	status := store.rows[j.ID].Status
	store.mu.Unlock()
	if status != models.JobRunning {
		t.Fatalf("false cancel status=%s", status)
	}
	close(release)
	awaitStatus(t, store.testStore, j.ID, models.JobCompleted)
}
