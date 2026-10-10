package handlers_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-ai-study/api/models"
	"go-ai-study/api/sourcefiles"
	"go-ai-study/internal/ai"
	"go-ai-study/internal/source"
)

func newBoundHTTPFixture(t *testing.T, missing bool) (*httpFixture, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "bound")
	if !missing {
		for _, directory := range []string{"Pkg", "sources", "volumes", ".runtime", "bin", "business/sources"} {
			if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(directory)), 0700); err != nil {
				t.Fatal(err)
			}
			code := "package sample\n//line virtual.go:900\nfunc Helper() {\n println(\"example\")\n}\n"
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(directory), "Code.go"), []byte(code), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	h := newHarness()
	settings := h.settings()
	settings.SourceBindings = []sourcefiles.Binding{{OwnerID: 7, ProjectID: 1, HostRoot: `F:\Repository`, ContainerRoot: root, ExcludePaths: []string{"sources", "volumes"}}}
	return newHTTPFixture(t, settings), root
}

func TestHTTPBoundSourceThreeOperationsAndPhysicalCitations(t *testing.T) {
	f, root := newBoundHTTPFixture(t, false)
	infoResponse := f.request(t, http.MethodGet, "/api/v1/projects/1/source", "", 7)
	var info sourcefiles.SourceInfo
	if infoResponse.Code != 200 || json.Unmarshal(infoResponse.Body.Bytes(), &info) != nil || !info.Available || !info.BindingConfigured || !info.AbsoluteInputAllowed || info.BindingID == "" || info.HostRoot != `F:\Repository` || infoResponse.Header().Get("Cache-Control") != "no-store" || strings.Contains(infoResponse.Body.String(), strings.ReplaceAll(root, "\\", "\\\\")) {
		t.Fatalf("source info=%d %s", infoResponse.Code, infoResponse.Body.String())
	}
	for _, target := range []string{`f:\REPOSITORY\pkg\code.go`, "Pkg/Code.go", `F:/Repository/Pkg/`} {
		body, _ := json.Marshal(map[string]string{"target": target})
		response := f.request(t, http.MethodPost, "/api/v1/projects/1/scan", string(body), 7)
		var scan source.ScanResult
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &scan) != nil || scan.TotalFiles != 1 || scan.Files[0] != "Pkg/Code.go" || strings.Contains(response.Body.String(), "Repository") {
			t.Fatalf("bound scan=%d %s", response.Code, response.Body.String())
		}
		job := f.await(t, f.submit(t, target))
		if job.Status != models.JobCompleted || strings.Contains(string(job.Result), "Repository") || strings.Contains(job.Target, "Repository") || !strings.Contains(string(job.Result), "Pkg/Code.go") {
			t.Fatalf("bound analyze=%+v", job)
		}
		job = f.await(t, submitIndex(t, f, 7, "1", target))
		var summary ai.IndexSummary
		if job.Status != models.JobCompleted || json.Unmarshal(job.Result, &summary) != nil || summary.FilesScanned != 1 || summary.TargetPath != job.Target || strings.Contains(job.Target, "Repository") {
			t.Fatalf("bound index=%+v", job)
		}
	}
	response := ask(t, f, 7, "1", "Explain Helper", "Pkg/Code.go")
	var answer ai.AnswerResult
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &answer) != nil {
		t.Fatalf("bound ask=%d %s", response.Code, response.Body.String())
	}
	found := false
	for _, citation := range answer.Sources {
		if citation.Source != "Pkg/Code.go" {
			t.Fatalf("absolute citation=%+v", citation)
		}
		if strings.Contains(citation.Content, "func Helper") {
			found = true
			if citation.StartLine != 2 || citation.EndLine != 5 {
				t.Fatalf("physical lines=%+v", citation)
			}
		}
	}
	if !found {
		t.Fatal("missing indexed physical citation")
	}
	if got := ask(t, f, 7, "1", "Explain Helper", `F:/Repository/Pkg/Code.go`); got.Code != 400 {
		t.Fatalf("ask accepted absolute filter=%d", got.Code)
	}
	// Bound root scans must not accidentally use the old SOURCE_ROOT tree.
	response = f.request(t, http.MethodPost, "/api/v1/projects/1/scan", `{}`, 7)
	var scan source.ScanResult
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &scan) != nil || strings.Join(scan.Files, ",") != "Pkg/Code.go,business/sources/Code.go" {
		t.Fatalf("bound root exclusions=%d %s", response.Code, response.Body.String())
	}
}

func TestHTTPBindingBoundaryErrorsAndOwnerIsolation(t *testing.T) {
	f, _ := newBoundHTTPFixture(t, false)
	for _, route := range []string{"scan", "analyze", "index"} {
		for _, target := range []string{`F:/RepositorySibling`, `F:/Repository/../Repository`, `F:Repository`, `F:/Repository/Pkg/Code.go:stream`, `\\host\share`, `\\?\F:\Repository`} {
			body, _ := json.Marshal(map[string]string{"target": target})
			if got := f.request(t, http.MethodPost, "/api/v1/projects/1/"+route, string(body), 7); got.Code != 400 || !strings.Contains(got.Body.String(), "invalid_target") {
				t.Fatalf("unsafe %s target=%d %s", route, got.Code, got.Body.String())
			}
		}
		for _, target := range []string{`F:/Repository/SOURCES`, "sources/Code.go", "volumes", ".runtime/Code.go", "bin/Code.go"} {
			body, _ := json.Marshal(map[string]string{"target": target})
			if got := f.request(t, http.MethodPost, "/api/v1/projects/1/"+route, string(body), 7); got.Code != 403 || !strings.Contains(got.Body.String(), "excluded_target") {
				t.Fatalf("excluded %s=%d %s", route, got.Code, got.Body.String())
			}
		}
		if got := f.request(t, http.MethodPost, "/api/v1/projects/1/"+route, `{"target":"F:/Repository"}`, 8); got.Code != 404 {
			t.Fatalf("foreign %s=%d", route, got.Code)
		}
	}
	for owner, status := range map[uint]int{0: 401, 8: 404} {
		if got := f.request(t, http.MethodGet, "/api/v1/projects/1/source", "", owner); got.Code != status || strings.Contains(got.Body.String(), "Repository") {
			t.Fatalf("foreign source info=%d %s", got.Code, got.Body.String())
		}
	}
	missing, _ := newBoundHTTPFixture(t, true)
	got := missing.request(t, http.MethodGet, "/api/v1/projects/1/source", "", 7)
	var info sourcefiles.SourceInfo
	if got.Code != 200 || json.Unmarshal(got.Body.Bytes(), &info) != nil || info.Available || info.HostRoot != `F:\Repository` || info.UnavailableCode != "source_unavailable" {
		t.Fatalf("missing info=%d %s", got.Code, got.Body.String())
	}
	if got := missing.request(t, http.MethodPost, "/api/v1/projects/1/scan", `{"target":"F:/Repository"}`, 7); got.Code != 422 {
		t.Fatalf("missing scan=%d", got.Code)
	}
	for _, id := range []string{missing.submit(t, `F:/Repository`), submitIndex(t, missing, 7, "1", `F:/Repository`)} {
		if job := missing.await(t, id); job.Status != models.JobFailed || job.FailureCode != "source_unavailable" || job.Target != "." {
			t.Fatalf("missing durable operation=%+v", job)
		}
	}
	legacy := newHTTPFixture(t)
	got = legacy.request(t, http.MethodGet, "/api/v1/projects/1/source", "", 7)
	info = sourcefiles.SourceInfo{}
	if got.Code != 200 || json.Unmarshal(got.Body.Bytes(), &info) != nil || info.BindingConfigured || info.AbsoluteInputAllowed || info.HostRoot != "" || !info.Available {
		t.Fatalf("legacy info=%d %s", got.Code, got.Body.String())
	}
	if got := legacy.request(t, http.MethodPost, "/api/v1/projects/1/scan", `{"target":"pkg/helper.go"}`, 7); got.Code != 200 {
		t.Fatalf("legacy relative changed=%d", got.Code)
	}
}
