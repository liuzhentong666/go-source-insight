package handlers_test

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"go-ai-study/api/handlers"
	"go-ai-study/api/models"
	"go-ai-study/api/routes"
	"go-ai-study/api/service"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func singleWorkerFixture(t *testing.T, h *ragHarness) *httpFixture {
	f := newHTTPFixture(t, h.settings())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := f.svc.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	svc, err := service.NewSource(context.Background(), f.repository, f.root, 1, 1, h.settings())
	if err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	f.router = gin.New()
	routes.RegisterRoutes(f.router, &handlers.SourceHandler{Service: svc, Tokens: f.signer})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := svc.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	if err := os.MkdirAll(filepath.Join(f.root, "8", "2"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "8", "2", "second.go"), []byte("package second\nfunc Second() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return f
}
func TestHTTPQueuedCancelReleasesBusyAndLegacySharesBound(t *testing.T) {
	h := newHarness()
	release := make(chan struct{})
	h.indexBlock = release
	f := singleWorkerFixture(t, h)
	first := submitIndex(t, f, 7, "1", ".")
	<-h.indexStarted
	second := submitIndex(t, f, 8, "2", ".")
	if got := f.request(t, http.MethodPost, "/api/v1/jobs/"+second+"/cancel", "{}", 7); got.Code != 404 {
		t.Fatalf("foreign cancel=%d %s", got.Code, got.Body.String())
	}
	if got := f.request(t, http.MethodPost, "/api/v1/jobs/"+second+"/cancel", "{}", 0); got.Code != 401 {
		t.Fatalf("unauthenticated cancel=%d", got.Code)
	}
	if got := f.request(t, http.MethodPost, "/api/v1/analysis/analyze", `{"project_id":1,"code":"package sample"}`, 7); got.Code != 503 || !strings.Contains(got.Body.String(), "queue_full") {
		t.Fatalf("legacy bypassed bound=%d %s", got.Code, got.Body.String())
	}
	if got := f.request(t, http.MethodPost, "/api/v1/jobs/"+second+"/cancel", "{}", 8); got.Code != 200 || !strings.Contains(got.Body.String(), "canceled") {
		t.Fatalf("queued cancel=%d %s", got.Code, got.Body.String())
	}
	if got := ask(t, f, 8, "2", "Second", ""); got.Code != 200 {
		t.Fatalf("queued cancel left busy=%d %s", got.Code, got.Body.String())
	}
	got := f.request(t, http.MethodPost, "/api/v1/analysis/analyze", `{"project_id":1,"code":"package sample\nfunc Legacy() {}"}`, 7)
	if got.Code != 202 {
		t.Fatalf("legacy admission=%d %s", got.Code, got.Body.String())
	}
	var accepted models.Job
	if err := json.Unmarshal(got.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.LegacyAnalysisID != 42 || accepted.ID == "" {
		t.Fatalf("lost legacy contract=%s", got.Body.String())
	}
	if got := f.request(t, http.MethodPost, "/api/v1/analysis/analyze", `{"project_id":1,"code":"package sample"}`, 8); got.Code != 404 {
		t.Fatalf("foreign legacy=%d", got.Code)
	}
	close(release)
	if job := f.await(t, first); job.Status != models.JobCompleted {
		t.Fatalf("first=%+v", job)
	}
	if job := f.await(t, accepted.ID); job.Status != models.JobCompleted || !strings.Contains(string(job.Result), "Legacy") {
		t.Fatalf("legacy result=%+v", job)
	}
	if got := f.request(t, http.MethodPost, "/api/v1/jobs/"+first+"/cancel", "{}", 7); got.Code != 200 || !strings.Contains(got.Body.String(), "completed") {
		t.Fatalf("terminal cancel=%d %s", got.Code, got.Body.String())
	}
}

func TestHTTPRunningIndexCancellationAndCommittedOutcome(t *testing.T) {
	t.Run("before publication", func(t *testing.T) {
		h := newHarness()
		h.indexBlock = make(chan struct{})
		f := newHTTPFixture(t, h.settings())
		id := submitIndex(t, f, 7, "1", ".")
		<-h.indexStarted
		if got := f.request(t, http.MethodPost, "/api/v1/jobs/"+id+"/cancel", "{}", 7); got.Code != 200 {
			t.Fatalf("running cancel=%d %s", got.Code, got.Body.String())
		}
		if job := f.await(t, id); job.Status != models.JobCanceled {
			t.Fatalf("cancel outcome=%+v", job)
		}
		if got := ask(t, f, 7, "1", "Helper", ""); got.Code != 200 {
			t.Fatalf("cancel left gate=%d", got.Code)
		}
		h.mu.Lock()
		calls, closed := len(h.opts), h.closed
		h.mu.Unlock()
		if calls != closed {
			t.Fatalf("clients leaked calls=%d closed=%d", calls, closed)
		}
	})
	t.Run("after publication", func(t *testing.T) {
		h := newHarness()
		release := make(chan struct{})
		h.afterPublish = release
		h.closeErr = true
		f := newHTTPFixture(t, h.settings())
		id := submitIndex(t, f, 7, "1", ".")
		<-h.published
		got := f.request(t, http.MethodPost, "/api/v1/jobs/"+id+"/cancel", "{}", 7)
		if got.Code != 200 || !strings.Contains(got.Body.String(), "canceling") {
			t.Fatalf("cancel request=%d %s", got.Code, got.Body.String())
		}
		close(release)
		if job := f.await(t, id); job.Status != models.JobCompleted {
			t.Fatalf("published snapshot mislabeled=%+v", job)
		}
		h.mu.Lock()
		h.closeErr = false
		h.mu.Unlock()
		if got := ask(t, f, 7, "1", "Helper", "pkg/helper.go"); got.Code != 200 || !strings.Contains(got.Body.String(), "pkg/helper.go") {
			t.Fatalf("published data missing=%d %s", got.Code, got.Body.String())
		}
	})
}
func TestHTTPShutdownCancelsModelsAndRejectsNewOperations(t *testing.T) {
	h := newHarness()
	f := newHTTPFixture(t, h.settings())
	id := submitIndex(t, f, 7, "1", ".")
	f.await(t, id)
	h.mu.Lock()
	h.askBlock = make(chan struct{})
	h.mu.Unlock()
	response := make(chan int, 1)
	go func() { response <- ask(t, f, 7, "1", "Helper", "").Code }()
	<-h.askStarted
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := f.svc.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if status := <-response; status != 408 {
		t.Fatalf("active model not canceled=%d", status)
	}
	for _, path := range []string{"scan", "analyze", "index", "ask"} {
		body := "{}"
		if path == "ask" {
			body = `{"question":"Helper"}`
		}
		if got := f.request(t, http.MethodPost, "/api/v1/projects/1/"+path, body, 7); got.Code != 503 || !strings.Contains(got.Body.String(), "service_stopping") {
			t.Fatalf("shutdown accepted %s=%d %s", path, got.Code, got.Body.String())
		}
	}
	h.mu.Lock()
	calls, closed := len(h.opts), h.closed
	h.mu.Unlock()
	if calls != closed {
		t.Fatalf("shutdown leaked clients=%d/%d", calls, closed)
	}
}
func TestAllJSONEntrypointsStrictAndBounded(t *testing.T) {
	f := newHTTPFixture(t)
	for _, path := range []string{"users/register", "users/login", "projects/", "analysis/analyze"} {
		for _, body := range []string{"null", `{"unknown":true}`, strings.Repeat(" ", 17<<10) + "{}", "{} {}"} {
			got := f.request(t, http.MethodPost, "/api/v1/"+path, body, 7)
			if got.Code != 400 {
				t.Fatalf("strict %s=%d %s", path, got.Code, got.Body.String())
			}
		}
	}
}
