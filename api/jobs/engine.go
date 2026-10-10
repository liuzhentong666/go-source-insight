// Package jobs executes persisted work with a fixed number of workers.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"go-ai-study/api/fault"
	"go-ai-study/api/models"
	"log"
	"sync"
	"time"
)

type Store interface {
	CreateJob(context.Context, *models.Job) error
	UpdateJob(context.Context, string, string, map[string]any) error
}
type Executor func(context.Context, models.Job) (json.RawMessage, error)
type entry struct {
	job     models.Job
	cancel  context.CancelFunc
	execute Executor
}
type Engine struct {
	mu       sync.Mutex
	ctx      context.Context
	store    Store
	execute  Executor
	pending  []*entry
	active   map[string]*entry
	wake     chan struct{}
	done     chan struct{}
	stopping bool
	capacity int
	timeout  time.Duration
	after    func(models.Job)
	wg       sync.WaitGroup
}

func New(ctx context.Context, store Store, execute Executor, workers, queueSize int, after ...func(models.Job)) (*Engine, error) {
	return NewTimed(ctx, store, execute, workers, queueSize, 15*time.Minute, after...)
}
func NewTimed(ctx context.Context, store Store, execute Executor, workers, queueSize int, timeout time.Duration, after ...func(models.Job)) (*Engine, error) {
	if ctx == nil || store == nil || execute == nil || workers < 1 || workers > 32 || queueSize < 1 || queueSize > 1024 || timeout <= 0 {
		return nil, fmt.Errorf("invalid job execution configuration")
	}
	e := &Engine{ctx: ctx, store: store, execute: execute, active: map[string]*entry{}, wake: make(chan struct{}, workers), done: make(chan struct{}), capacity: workers + queueSize, timeout: timeout}
	if len(after) > 0 {
		e.after = after[0]
	}
	e.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go e.worker()
	}
	go func() { e.wg.Wait(); close(e.done) }()
	go func() {
		select {
		case <-ctx.Done():
			e.Stop()
		case <-e.done:
		}
	}()
	return e, nil
}
func (e *Engine) Submit(ctx context.Context, ownerID, projectID uint, kind, target string) (models.Job, error) {
	return e.SubmitJob(ctx, models.Job{OwnerID: ownerID, ProjectID: projectID, Kind: kind, Target: target})
}

// Serialize admission, persistence and cancellation so no accepted job escapes
// the capacity bound or begins after a durable queued cancellation.
func (e *Engine) SubmitJob(ctx context.Context, job models.Job) (models.Job, error) {
	return e.SubmitWithExecutor(ctx, job, nil)
}

// SubmitWithExecutor captures execution context before persistence and admission.
// Only the durable Job is stored; startup recovery never replays this closure.
func (e *Engine) SubmitWithExecutor(ctx context.Context, job models.Job, execute Executor) (models.Job, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopping || e.ctx.Err() != nil {
		return models.Job{}, fault.ErrStopping
	}
	if len(e.active) >= e.capacity {
		return models.Job{}, fault.ErrQueueFull
	}
	job.ID = uuid.NewString()
	job.Status = models.JobQueued
	if err := e.store.CreateJob(ctx, &job); err != nil {
		return models.Job{}, err
	}
	item := &entry{job: job, execute: execute}
	e.active[job.ID] = item
	e.pending = append(e.pending, item)
	e.signal()
	return job, nil
}
func (e *Engine) signal() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
}
func (e *Engine) finish(item *entry) {
	delete(e.active, item.job.ID)
	if e.after != nil {
		e.after(item.job)
	}
}

// Terminal cancellation is idempotent. Running cancellation is only a request;
// a successful executor result still determines the final durable outcome.
func (e *Engine) Cancel(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	item := e.active[id]
	if item == nil {
		return nil
	}
	if item.cancel == nil {
		if err := e.update(id, models.JobQueued, map[string]any{"status": models.JobCanceled, "failure_code": "canceled", "failure_reason": "operation canceled", "finished_at": time.Now().UTC()}); err != nil {
			return err
		}
		for i, p := range e.pending {
			if p == item {
				e.pending = append(e.pending[:i], e.pending[i+1:]...)
				break
			}
		}
		e.finish(item)
		return nil
	}
	if item.job.Status == models.JobCanceling {
		return nil
	}
	if err := e.update(id, models.JobRunning, map[string]any{"status": models.JobCanceling}); err != nil {
		return err
	}
	item.job.Status = models.JobCanceling
	item.cancel()
	return nil
}
func (e *Engine) worker() {
	defer e.wg.Done()
	for {
		e.mu.Lock()
		if e.stopping {
			e.mu.Unlock()
			return
		}
		if len(e.pending) == 0 {
			e.mu.Unlock()
			select {
			case <-e.ctx.Done():
				e.Stop()
			case <-e.wake:
			}
			continue
		}
		item := e.pending[0]
		e.pending = e.pending[1:]
		if len(e.pending) > 0 {
			e.signal()
		}
		if err := e.update(item.job.ID, models.JobQueued, map[string]any{"status": models.JobRunning, "started_at": time.Now().UTC()}); err != nil {
			e.finish(item)
			e.mu.Unlock()
			continue
		}
		item.job.Status = models.JobRunning
		ctx, cancel := context.WithTimeout(e.ctx, e.timeout)
		item.cancel = cancel
		job := item.job
		e.mu.Unlock()
		execute := item.execute
		if execute == nil {
			execute = e.execute
		}
		result, err := e.call(ctx, job, execute)
		cancel()
		e.mu.Lock()
		changes := map[string]any{"status": models.JobCompleted, "result": string(result), "finished_at": time.Now().UTC()}
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				err = fault.ErrTimeout
			}
			if errors.Is(err, context.Canceled) {
				err = fault.ErrCanceled
			}
			code, message := fault.Public(err)
			status := models.JobFailed
			if code == "canceled" {
				status = models.JobCanceled
			}
			changes["status"] = status
			changes["result"] = nil
			changes["failure_code"] = code
			changes["failure_reason"] = message
		}
		// A successful executor result wins a simultaneous cancel request, including
		// an index whose snapshot has already been published.
		_ = e.update(item.job.ID, item.job.Status, changes)
		e.finish(item)
		e.mu.Unlock()
	}
}
func (e *Engine) call(ctx context.Context, job models.Job, execute Executor) (result json.RawMessage, err error) {
	defer func() {
		if recover() != nil {
			result = nil
			err = fmt.Errorf("job execution failed")
		}
	}()
	result, err = execute(ctx, job)
	if err == nil && !json.Valid(result) {
		return nil, fmt.Errorf("job returned invalid result")
	}
	return
}
func (e *Engine) update(id, expected string, changes map[string]any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := e.store.UpdateJob(ctx, id, expected, changes)
	if err != nil {
		code, _ := fault.Public(err)
		log.Printf("job state persistence failed job=%s code=%s; startup reconciliation required", id, code)
	}
	return err
}

// Failed shutdown writes remain nonterminal for startup reconciliation.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopping {
		return
	}
	e.stopping = true
	for _, item := range e.active {
		if item.cancel != nil {
			item.cancel()
		}
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, item := range e.pending {
		if err := e.store.UpdateJob(cleanupCtx, item.job.ID, models.JobQueued, map[string]any{"status": models.JobCanceled, "failure_code": "shutdown", "failure_reason": "service stopped before execution", "finished_at": time.Now().UTC()}); err != nil {
			log.Printf("queued shutdown persistence failed job=%s; startup reconciliation required", item.job.ID)
		}
		e.finish(item)
	}
	e.pending = nil
	for i := 0; i < cap(e.wake); i++ {
		e.signal()
	}
}
func (e *Engine) Shutdown(ctx context.Context) error {
	go e.Stop()
	select {
	case <-e.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
