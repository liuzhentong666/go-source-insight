package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-ai-study/api/fault"
	"go-ai-study/api/models"
)

func TestCapturedExecutorQueueCancellationAndShutdown(t *testing.T) {
	store := newStore()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	closed := make(chan struct{})
	var defaultCalls, capturedCalls atomic.Int32
	e, err := New(ctx, store, func(context.Context, models.Job) (json.RawMessage, error) {
		defaultCalls.Add(1)
		return json.RawMessage(`{}`), nil
	}, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	first, err := e.SubmitWithExecutor(ctx, models.Job{OwnerID: 7, ProjectID: 1, Kind: models.JobIndex}, func(ctx context.Context, _ models.Job) (json.RawMessage, error) {
		close(started)
		<-ctx.Done()
		close(closed)
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	queued, err := e.SubmitWithExecutor(ctx, models.Job{OwnerID: 7, ProjectID: 2, Kind: models.JobIndex}, func(context.Context, models.Job) (json.RawMessage, error) {
		capturedCalls.Add(1)
		return json.RawMessage(`{}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Cancel(queued.ID); err != nil {
		t.Fatal(err)
	}
	awaitStatus(t, store, queued.ID, models.JobCanceled)
	shutdownCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	if err := e.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-closed:
	default:
		t.Fatal("shutdown returned before executor resource closure")
	}
	awaitStatus(t, store, first.ID, models.JobCanceled)
	if capturedCalls.Load() != 0 || defaultCalls.Load() != 0 {
		t.Fatal("wrong or canceled executor ran")
	}
}

func TestCapturedExecutorConcurrentAdmissionAndPersistenceFailure(t *testing.T) {
	store := newStore()
	store.createErr = fault.ErrStorage
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var defaults, captured atomic.Int32
	e, err := New(ctx, store, func(context.Context, models.Job) (json.RawMessage, error) {
		defaults.Add(1)
		return json.RawMessage(`{}`), nil
	}, 4, 16)
	if err != nil {
		t.Fatal(err)
	}
	execute := func(context.Context, models.Job) (json.RawMessage, error) {
		captured.Add(1)
		return json.RawMessage(`{"captured":true}`), nil
	}
	if _, err := e.SubmitWithExecutor(ctx, models.Job{}, execute); !errors.Is(err, fault.ErrStorage) {
		t.Fatalf("persistence=%v", err)
	}
	store.mu.Lock()
	store.createErr = nil
	store.mu.Unlock()
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.SubmitWithExecutor(ctx, models.Job{OwnerID: 7, ProjectID: 1, Kind: models.JobAnalyze}, execute); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	// Observe every durable completion; executions may race admissions.
	count := 0
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for count < 12 {
		select {
		case j := <-store.events:
			if j.Status == models.JobCompleted {
				if string(j.Result) != `{"captured":true}` {
					t.Fatalf("wrong executor=%s", j.Result)
				}
				count++
			}
		case <-timer.C:
			t.Fatal("captured executors did not drain")
		}
	}
	if captured.Load() != 12 || defaults.Load() != 0 {
		t.Fatalf("calls captured=%d default=%d", captured.Load(), defaults.Load())
	}
	stopCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	if err := e.Shutdown(stopCtx); err != nil {
		t.Fatal(err)
	}
	// Constructing a replacement engine never loads durable rows or saved closures.
	restarted, err := New(ctx, store, execute, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Shutdown(stopCtx); err != nil {
		t.Fatal(err)
	}
	if captured.Load() != 12 {
		t.Fatal("replacement engine replayed prior work")
	}
}
