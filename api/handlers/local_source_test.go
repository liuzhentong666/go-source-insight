package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go-ai-study/api/handlers"
	"go-ai-study/api/models"
	"go-ai-study/api/routes"
	"go-ai-study/api/service"
	"go-ai-study/api/sourcefiles"
	"go-ai-study/internal/source"
)

func localHTTPFixture(t *testing.T, h *ragHarness, missing bool) *httpFixture {
	t.Helper()
	local := filepath.Join(t.TempDir(), "local")
	if !missing {
		for name, code := range map[string]string{"Pkg/Local.go": "package sample\n//line virtual.go:900\nfunc Local() {}\n", "Pkg/Local_test.go": "package sample\nfunc TestLocal() {}\n", "sources/private.go": "invalid source"} {
			file := filepath.Join(local, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte(code), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	bound := t.TempDir()
	if err := os.WriteFile(filepath.Join(bound, "bound.go"), []byte("package bound\nfunc Bound() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	settings := h.settings()
	settings.LocalRoot = &sourcefiles.LocalRoot{HostRoot: `F:\Local`, ContainerRoot: local, ExcludePaths: []string{"sources"}}
	settings.SourceBindings = []sourcefiles.Binding{{OwnerID: 7, ProjectID: 1, HostRoot: `F:\Bound`, ContainerRoot: bound}}
	f := newHTTPFixture(t, settings)
	// One worker makes queued scope capture observable without timing assumptions.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := f.svc.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	svc, err := service.NewSource(context.Background(), f.repository, f.root, 1, 3, settings)
	if err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	f.router = gin.New()
	routes.RegisterRoutes(f.router, &handlers.SourceHandler{Service: svc, Tokens: f.signer})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := svc.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	if err := os.MkdirAll(filepath.Join(f.root, "8", "2"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "8", "2", "legacy.go"), []byte("package legacy\nfunc Legacy() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestHTTPLocalScopesRemainSelectedWhenQueuedAndRelativeStaysOld(t *testing.T) {
	h := newHarness()
	h.timeout = 10
	release := make(chan struct{})
	f := localHTTPFixture(t, h, false)
	for _, tc := range []struct {
		project, owner uint
		target, want   string
	}{{1, 7, ".", "bound.go"}, {2, 8, ".", "legacy.go"}, {1, 7, `F:/Local/Pkg`, "Pkg/Local.go,Pkg/Local_test.go"}, {2, 8, `f:\local\pkg`, "Pkg/Local.go,Pkg/Local_test.go"}} {
		body, _ := json.Marshal(map[string]string{"target": tc.target})
		got := f.request(t, http.MethodPost, "/api/v1/projects/"+strconv.FormatUint(uint64(tc.project), 10)+"/scan", string(body), tc.owner)
		var scan source.ScanResult
		if got.Code != 200 || json.Unmarshal(got.Body.Bytes(), &scan) != nil || strings.Join(scan.Files, ",") != tc.want {
			t.Fatalf("selected scan=%d %s", got.Code, got.Body.String())
		}
	}
	for _, tc := range []struct {
		project string
		owner   uint
	}{{"1", 7}, {"2", 8}} {
		got := f.request(t, http.MethodGet, "/api/v1/projects/"+tc.project+"/source", "", tc.owner)
		var info sourcefiles.SourceInfo
		if got.Code != 200 || json.Unmarshal(got.Body.Bytes(), &info) != nil || info.SourceMode != "local" || !info.Available || info.HostRoot != `F:\Local` {
			t.Fatalf("local info=%d %s", got.Code, got.Body.String())
		}
	}

	// Local configuration never changes relative analyze/index execution roots.
	for _, tc := range []struct {
		project string
		owner   uint
		source  string
	}{{"1", 7, "bound.go"}, {"2", 8, "legacy.go"}} {
		acceptedResponse := f.request(t, http.MethodPost, "/api/v1/projects/"+tc.project+"/analyze", `{"target":"."}`, tc.owner)
		var accepted struct {
			ID string `json:"job_id"`
		}
		if acceptedResponse.Code != 202 || json.Unmarshal(acceptedResponse.Body.Bytes(), &accepted) != nil {
			t.Fatalf("relative analyze=%d %s", acceptedResponse.Code, acceptedResponse.Body.String())
		}
		if job := f.awaitOwner(t, accepted.ID, tc.owner); job.Status != models.JobCompleted || !strings.Contains(string(job.Result), tc.source) || strings.Contains(string(job.Result), "Local.go") {
			t.Fatalf("relative analyze switched root=%+v", job)
		}
		id := submitIndex(t, f, tc.owner, tc.project, ".")
		if job := f.awaitOwner(t, id, tc.owner); job.Status != models.JobCompleted {
			t.Fatalf("relative index=%+v", job)
		}
		got := ask(t, f, tc.owner, tc.project, "Explain code", tc.source)
		if got.Code != 200 || !strings.Contains(got.Body.String(), tc.source) {
			t.Fatalf("relative index switched root=%d %s", got.Code, got.Body.String())
		}
	}
	h.mu.Lock()
	h.indexBlock = release
	h.mu.Unlock()
	// Relative indexes above emitted start notifications; drain those before blocking.
	<-h.indexStarted
	<-h.indexStarted
	first := submitIndex(t, f, 7, "1", `F:/Local/Pkg`)
	<-h.indexStarted
	queuedAnalyze := f.request(t, http.MethodPost, "/api/v1/projects/2/analyze", `{"target":"f:/local/pkg/local.go"}`, 8)
	if queuedAnalyze.Code != 202 {
		t.Fatalf("queue analyze=%d %s", queuedAnalyze.Code, queuedAnalyze.Body.String())
	}
	var accepted struct {
		ID string `json:"job_id"`
	}
	if err := json.Unmarshal(queuedAnalyze.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	second := submitIndex(t, f, 8, "2", `F:/Local/Pkg/Local.go`)
	for _, id := range []string{accepted.ID, second} {
		got := f.request(t, http.MethodGet, "/api/v1/jobs/"+id, "", 8)
		if !strings.Contains(got.Body.String(), `"status":"queued"`) || !strings.Contains(got.Body.String(), `"target":"Pkg/Local.go"`) {
			t.Fatalf("not canonically queued: %s", got.Body.String())
		}
	}
	close(release)
	if job := f.await(t, first); job.Status != models.JobCompleted {
		t.Fatalf("first=%+v", job)
	}
	if job := f.awaitOwner(t, accepted.ID, 8); job.Status != models.JobCompleted || !strings.Contains(string(job.Result), "Pkg/Local.go") || strings.Contains(string(job.Result), "legacy.go") {
		t.Fatalf("queued analyze lost root=%+v", job)
	}
	if job := f.awaitOwner(t, second, 8); job.Status != models.JobCompleted {
		t.Fatalf("queued index lost root=%+v", job)
	}
	answer := ask(t, f, 8, "2", "Explain Local", "Pkg/Local.go")
	if answer.Code != 200 || !strings.Contains(answer.Body.String(), "func Local") || !strings.Contains(answer.Body.String(), `"start_line":2`) {
		t.Fatalf("relative physical citation=%d %s", answer.Code, answer.Body.String())
	}
	for _, route := range []string{"source", "scan", "analyze", "index"} {
		method := http.MethodPost
		body := `{"target":"F:/Local"}`
		if route == "source" {
			method = http.MethodGet
			body = ""
		}
		if got := f.request(t, method, "/api/v1/projects/1/"+route, body, 8); got.Code != 404 {
			t.Fatalf("local ownership %s=%d", route, got.Code)
		}
	}
}

func TestHTTPLocalMissingRootDurablyFailsWithoutFallback(t *testing.T) {
	f := localHTTPFixture(t, newHarness(), true)
	got := f.request(t, http.MethodGet, "/api/v1/projects/1/source", "", 7)
	if got.Code != 200 || !strings.Contains(got.Body.String(), `"source_mode":"local"`) || !strings.Contains(got.Body.String(), `"available":false`) {
		t.Fatalf("missing info=%d %s", got.Code, got.Body.String())
	}
	if got := f.request(t, http.MethodPost, "/api/v1/projects/1/scan", `{"target":"F:/Local"}`, 7); got.Code != 422 {
		t.Fatalf("missing scan=%d", got.Code)
	}
	for _, id := range []string{f.submit(t, `F:/Local`), submitIndex(t, f, 7, "1", `F:/Local`)} {
		if job := f.await(t, id); job.Status != models.JobFailed || job.FailureCode != "source_unavailable" {
			t.Fatalf("missing durable local=%+v", job)
		}
	}
	if got := f.request(t, http.MethodPost, "/api/v1/projects/1/scan", `{}`, 7); got.Code != 200 || !strings.Contains(got.Body.String(), "bound.go") {
		t.Fatalf("relative unavailable-local fallback semantics changed=%d %s", got.Code, got.Body.String())
	}
	for _, route := range []string{"scan", "analyze", "index"} {
		if got := f.request(t, http.MethodPost, "/api/v1/projects/1/"+route, `{"target":"F:/Bound"}`, 7); got.Code != 400 {
			t.Fatalf("bound absolute bypass %s=%d", route, got.Code)
		}
	}
}

func TestHTTPLocalQueuedExecutionRechecksOwnership(t *testing.T) {
	h := newHarness()
	h.timeout = 10
	release := make(chan struct{})
	h.indexBlock = release
	f := localHTTPFixture(t, h, false)
	first := submitIndex(t, f, 7, "1", `F:/Local/Pkg`)
	<-h.indexStarted
	second := submitIndex(t, f, 8, "2", `F:/Local/Pkg`)
	f.repository.mu.Lock()
	f.repository.projects[2] = models.Project{ID: 2, OwnerID: 7}
	f.repository.mu.Unlock()
	close(release)
	if job := f.await(t, first); job.Status != models.JobCompleted {
		t.Fatalf("blocking index=%+v", job)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		f.repository.mu.Lock()
		job := f.repository.jobs[second]
		f.repository.mu.Unlock()
		if job.Status == models.JobFailed {
			if job.FailureCode != "not_found" {
				t.Fatalf("worker ownership failure=%+v", job)
			}
			h.mu.Lock()
			_, indexed := h.collections[service.ProjectCollection(2)]
			h.mu.Unlock()
			if indexed {
				t.Fatal("ownership changed but captured index still ran")
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("queued local job did not fail after ownership changed")
}

func TestHTTPLocalQueuedCancelAndShutdownCloseResources(t *testing.T) {
	h := newHarness()
	h.timeout = 10
	h.indexBlock = make(chan struct{})
	f := localHTTPFixture(t, h, false)
	first := submitIndex(t, f, 7, "1", `F:/Local/Pkg`)
	<-h.indexStarted
	second := submitIndex(t, f, 8, "2", `F:/Local/Pkg`)
	got := f.request(t, http.MethodPost, "/api/v1/jobs/"+second+"/cancel", `{}`, 8)
	if got.Code != 200 || !strings.Contains(got.Body.String(), `"status":"canceled"`) {
		t.Fatalf("local queued cancel=%d %s", got.Code, got.Body.String())
	}
	// Cancellation releases the project reservation without running its scanner.
	if got := ask(t, f, 8, "2", "Explain Local", ""); got.Code != 200 {
		t.Fatalf("local queue cancellation left project busy=%d %s", got.Code, got.Body.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := f.svc.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if job := f.await(t, first); job.Status != models.JobCanceled {
		t.Fatalf("local shutdown=%+v", job)
	}
	h.mu.Lock()
	closed := h.closed
	_, indexed := h.collections[service.ProjectCollection(2)]
	h.mu.Unlock()
	if closed < 1 || indexed {
		t.Fatalf("closure=%d canceled local index ran=%v", closed, indexed)
	}
}
