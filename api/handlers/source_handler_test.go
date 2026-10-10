package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go-ai-study/api/fault"
	"go-ai-study/api/handlers"
	"go-ai-study/api/models"
	"go-ai-study/api/routes"
	"go-ai-study/api/service"
	"go-ai-study/api/utils"
)

type memoryRepository struct {
	mu       sync.Mutex
	projects map[uint]models.Project
	jobs     map[string]models.Job
	readyErr error
}

func (r *memoryRepository) Ready(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.readyErr
}

func (r *memoryRepository) OwnedProject(_ context.Context, id, owner uint) (models.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.projects[id]
	if !ok || p.OwnerID != owner {
		return models.Project{}, fault.ErrNotFound
	}
	return p, nil
}
func (r *memoryRepository) OwnedJob(_ context.Context, id string, owner uint) (models.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	p, exists := r.projects[j.ProjectID]
	if !ok || !exists || j.OwnerID != owner || p.OwnerID != owner {
		return models.Job{}, fault.ErrNotFound
	}
	return j, nil
}
func (r *memoryRepository) CreateJob(_ context.Context, j *models.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	j.CreatedAt = time.Now().UTC()
	if j.Kind == models.JobLegacyAnalyze {
		j.LegacyAnalysisID = 42
	}
	j.UpdatedAt = j.CreatedAt
	r.jobs[j.ID] = *j
	return nil
}
func (r *memoryRepository) UpdateJob(_ context.Context, id, expected string, changes map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok || j.Status != expected {
		return fault.ErrConflict
	}
	j.Status = changes["status"].(string)
	if v, ok := changes["started_at"].(time.Time); ok {
		j.StartedAt = &v
	}
	if v, ok := changes["finished_at"].(time.Time); ok {
		j.FinishedAt = &v
	}
	if v, ok := changes["result"].(string); ok {
		j.Result = json.RawMessage(v)
	}
	if v, ok := changes["failure_code"].(string); ok {
		j.FailureCode = v
	}
	if v, ok := changes["failure_reason"].(string); ok {
		j.FailureReason = v
	}
	j.UpdatedAt = time.Now().UTC()
	r.jobs[id] = j
	return nil
}

type httpFixture struct {
	router     *gin.Engine
	root       string
	repository *memoryRepository
	tokens     map[uint]string
	signer     *utils.Tokens
	svc        *service.Source
}

func newHTTPFixture(t *testing.T, ragSettings ...service.RAGSettings) *httpFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	f := &httpFixture{root: t.TempDir(), repository: &memoryRepository{projects: map[uint]models.Project{1: {ID: 1, OwnerID: 7}, 2: {ID: 2, OwnerID: 8}}, jobs: map[string]models.Job{}}, tokens: map[uint]string{}}
	projectRoot := filepath.Join(f.root, "7", "1")
	if err := os.MkdirAll(filepath.Join(projectRoot, "pkg"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, code := range map[string]string{
		"main.go":       "package sample\n//line mapped.go:900\nconst password = \"example-placeholder\"\nfunc read(){ f, _ := os.Open(\"sample.txt\"); _ = f }\n",
		"main_test.go":  "package sample\nfunc TestIgnored() {}\n",
		"pkg/helper.go": "package sample\nfunc Helper() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(projectRoot, filepath.FromSlash(name)), []byte(code), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".git", "ignored.go"), []byte("invalid source"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	workers := 1
	if len(ragSettings) > 0 {
		workers = 2
	}
	svc, err := service.NewSource(ctx, f.repository, f.root, workers, 2, ragSettings...)
	if err != nil {
		t.Fatal(err)
	}
	f.router = gin.New()
	f.svc = svc
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = svc.Shutdown(ctx)
	})
	signer, err := utils.NewTokens("N7q4KP2rV8mZ1uX5dT9sB3cW6fH0jLQe")
	if err != nil {
		t.Fatal(err)
	}
	routes.RegisterRoutes(f.router, &handlers.SourceHandler{Service: svc, Tokens: signer})
	f.signer = signer
	for _, owner := range []uint{7, 8} {
		token, err := signer.Generate(models.User{ID: owner, Username: "example-user"})
		if err != nil {
			t.Fatal(err)
		}
		f.tokens[owner] = token
	}
	return f
}

func (f *httpFixture) request(t *testing.T, method, url, body string, owner uint) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if owner != 0 {
		req.Header.Set("Authorization", "Bearer "+f.tokens[owner])
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), f.root) || strings.Contains(w.Body.String(), strings.ReplaceAll(f.root, "\\", "\\\\")) {
		t.Fatalf("host path leaked: %s", w.Body.String())
	}
	return w
}

func TestSourceHTTPScanAndAccessBoundaries(t *testing.T) {
	f := newHTTPFixture(t)
	for _, url := range []string{"/api/v1/projects/1/scan", "/api/v1/projects/1/analyze", "/api/v1/jobs/00000000-0000-0000-0000-000000000000"} {
		method := http.MethodPost
		if strings.Contains(url, "/jobs/") {
			method = http.MethodGet
		}
		if got := f.request(t, method, url, "", 0); got.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s=%d", url, got.Code)
		}
	}
	for _, route := range []string{"scan", "analyze"} {
		if got := f.request(t, http.MethodPost, "/api/v1/projects/1/"+route, "{}", 8); got.Code != http.StatusNotFound {
			t.Fatalf("wrong owner %s=%d", route, got.Code)
		}
	}
	for _, target := range []string{"../2", "/outside", "C:/outside", "C:\\outside", "pkg/../../outside", "pkg\\helper.go", "main.go:stream", "NUL", "pkg/.."} {
		for _, route := range []string{"scan", "analyze"} {
			body, _ := json.Marshal(map[string]string{"target": target})
			got := f.request(t, http.MethodPost, "/api/v1/projects/1/"+route, string(body), 7)
			if got.Code != http.StatusBadRequest {
				t.Fatalf("target %q %s=%d %s", target, route, got.Code, got.Body.String())
			}
		}
	}
	for _, body := range []string{`{"path":"outside"}`, `null`, `{} {}`, `{"target":3}`, strings.Repeat(" ", 17<<10) + `{}`} {
		if got := f.request(t, http.MethodPost, "/api/v1/projects/1/scan", body, 7); got.Code != http.StatusBadRequest {
			t.Fatalf("invalid body accepted: %d", got.Code)
		}
	}
	got := f.request(t, http.MethodPost, "/api/v1/projects/1/scan", "", 7)
	if got.Code != http.StatusOK {
		t.Fatalf("scan=%d %s", got.Code, got.Body.String())
	}
	var report struct {
		Target     string   `json:"target_path"`
		Total      int      `json:"total_files"`
		Production int      `json:"production_file_count"`
		Tests      int      `json:"test_file_count"`
		Files      []string `json:"files"`
		Skipped    []string `json:"skipped_dirs"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Target != "." || report.Total != 3 || report.Production != 2 || report.Tests != 1 || strings.Join(report.Files, ",") != "main.go,main_test.go,pkg/helper.go" || strings.Join(report.Skipped, ",") != ".git" {
		t.Fatalf("wrong relative scan: %+v", report)
	}
	got = f.request(t, http.MethodPost, "/api/v1/projects/1/scan", `{"target":"pkg"}`, 7)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "pkg/helper.go") {
		t.Fatalf("nested scan: %d %s", got.Code, got.Body.String())
	}
	got = f.request(t, http.MethodPost, "/api/v1/projects/1/scan", `{"target":"missing"}`, 7)
	if got.Code != http.StatusUnprocessableEntity || !strings.Contains(got.Body.String(), "source_unavailable") {
		t.Fatalf("failed scan=%d %s", got.Code, got.Body.String())
	}
	for _, body := range []string{`{"name":"example","path":"outside"}`, `{"name":"example","source_root":"outside"}`, `{"name":"example","target":"outside"}`} {
		if got := f.request(t, http.MethodPost, "/api/v1/projects/", body, 7); got.Code != http.StatusBadRequest {
			t.Fatalf("project accepted path metadata: %d %s", got.Code, got.Body.String())
		}
	}
}

func (f *httpFixture) submit(t *testing.T, target string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"target": target})
	w := f.request(t, http.MethodPost, "/api/v1/projects/1/analyze", string(body), 7)
	if w.Code != http.StatusAccepted {
		t.Fatalf("analyze=%d %s", w.Code, w.Body.String())
	}
	var accepted struct {
		ID     string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.ID == "" || accepted.Status != models.JobQueued || w.Header().Get("Location") != "/api/v1/jobs/"+accepted.ID {
		t.Fatalf("invalid acceptance: %s", w.Body.String())
	}
	return accepted.ID
}

func (f *httpFixture) await(t *testing.T, id string) models.Job {
	return f.awaitOwner(t, id, 7)
}

func (f *httpFixture) awaitOwner(t *testing.T, id string, owner uint) models.Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		w := f.request(t, http.MethodGet, "/api/v1/jobs/"+id, "", owner)
		if w.Code != http.StatusOK {
			t.Fatalf("get job=%d %s", w.Code, w.Body.String())
		}
		var job models.Job
		if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
			t.Fatal(err)
		}
		if job.Status == models.JobCompleted || job.Status == models.JobFailed || job.Status == models.JobCanceled {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job did not reach a terminal state")
	return models.Job{}
}

func TestSourceHTTPAnalysisJobsAndFailures(t *testing.T) {
	f := newHTTPFixture(t)
	id := f.submit(t, "")
	job := f.await(t, id)
	if job.Status != models.JobCompleted || job.StartedAt == nil || job.FinishedAt == nil || !json.Valid(job.Result) {
		t.Fatalf("invalid completed job: %+v", job)
	}
	var result struct {
		Files     int `json:"files_analyzed"`
		Functions int `json:"functions_analyzed"`
		Security  int `json:"security_issues"`
		Bugs      int `json:"bug_issues"`
	}
	if err := json.Unmarshal(job.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Files != 2 || result.Functions != 2 || result.Security < 1 || result.Bugs < 1 {
		t.Fatalf("unexpected shared analysis: %+v", result)
	}
	if got := f.request(t, http.MethodGet, "/api/v1/jobs/"+id, "", 8); got.Code != http.StatusNotFound {
		t.Fatalf("other owner read job=%d", got.Code)
	}
	for target, code := range map[string]string{"missing": "source_unavailable", "broken.go": "invalid_source"} {
		if target == "broken.go" {
			if err := os.WriteFile(filepath.Join(f.root, "7", "1", target), []byte("package sample\nfunc {"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		failed := f.await(t, f.submit(t, target))
		if failed.Status != models.JobFailed || failed.FailureCode != code || failed.FailureReason == "" || string(failed.Result) != "null" {
			t.Fatalf("wrong failed job: %+v", failed)
		}
	}
	for _, url := range []string{"/api/v1/jobs/00000000-0000-0000-0000-000000000000", "/api/v1/projects/99/scan"} {
		method := http.MethodGet
		if strings.Contains(url, "/projects/") {
			method = http.MethodPost
		}
		if got := f.request(t, method, url, "{}", 7); got.Code != http.StatusNotFound {
			t.Fatalf("missing resource=%d", got.Code)
		}
	}
	f.repository.mu.Lock()
	delete(f.repository.projects, 1)
	f.repository.mu.Unlock()
	if got := f.request(t, http.MethodGet, "/api/v1/jobs/"+id, "", 7); got.Code != http.StatusNotFound {
		t.Fatalf("deleted project job exposed=%d", got.Code)
	}
}

type fullQueue struct{ handlers.SourceService }

func (s fullQueue) SubmitAnalyze(context.Context, uint, uint, string) (models.Job, error) {
	return models.Job{}, fault.ErrQueueFull
}

func TestSourceHTTPQueueFullAndSafeErrors(t *testing.T) {
	f := newHTTPFixture(t)
	f.router = gin.New()
	routes.RegisterRoutes(f.router, &handlers.SourceHandler{Service: fullQueue{}, Tokens: f.signer})
	w := f.request(t, http.MethodPost, "/api/v1/projects/1/analyze", "{}", 7)
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" || !strings.Contains(w.Body.String(), "queue_full") {
		t.Fatalf("queue-full response=%d %s", w.Code, w.Body.String())
	}
	// Arbitrary internal details must never be used as public failure text.
	code, message := fault.Public(errors.New("internal file path and database connection details"))
	if code != "operation_failed" || message != "operation failed" {
		t.Fatalf("unsafe public error: %s %s", code, message)
	}
}
