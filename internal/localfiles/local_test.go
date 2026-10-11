package localfiles

import (
	"context"
	"errors"
	"go-ai-study/internal/source"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHostSources(t *testing.T) {
	token := strings.Repeat("a", 48)
	server := httptest.NewServer(Handler(token))
	defer server.Close()
	root := t.TempDir()
	other := t.TempDir()
	for _, dir := range []string{root, other} {
		content := "package main\nfunc main() {}\n"
		if dir == other {
			content = "package other\n"
		}
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(root, "neighbor.go"), []byte("package main\n"), 0600)
	os.WriteFile(filepath.Join(root, "secret.txt"), []byte("private"), 0600)
	ignoredDir := "vendor"
	if runtime.GOOS == "windows" {
		ignoredDir = "Vendor"
	}
	os.MkdirAll(filepath.Join(root, ignoredDir), 0700)
	os.WriteFile(filepath.Join(root, ignoredDir, "ignored.go"), []byte("package ignored"), 0600)
	for _, provider := range []Provider{{}, {URL: server.URL, Token: token}} {
		if !provider.Healthy(context.Background()) {
			t.Fatal("authenticated health request failed")
		}
		for _, dir := range []string{root, other} {
			err := provider.WithFS(context.Background(), filepath.Join(dir, "main.go"), func(files fs.FS, target string) error {
				result, err := source.ScanFS(context.Background(), files, target)
				if err != nil {
					return err
				}
				if result.TotalFiles != 1 || result.Files[0] != "main.go" {
					t.Fatalf("single file scope expanded: %+v", result)
				}
				expectedLines := 2
				if dir == other {
					expectedLines = 1
				}
				if result.TotalLines != expectedLines {
					t.Fatalf("cross-directory source mix: %+v", result)
				}
				if _, err := files.Open("neighbor.go"); err == nil {
					t.Fatal("sibling readable")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		err := provider.WithFS(context.Background(), root, func(files fs.FS, target string) error {
			result, err := source.ScanFS(context.Background(), files, target)
			if err == nil && result.TotalFiles != 2 {
				t.Fatalf("directory: %+v", result)
			}
			if err == nil && (len(result.SkippedDirs) != 1 || result.SkippedDirs[0] != ignoredDir) {
				t.Fatalf("host directory policy lost: %+v", result)
			}
			if _, err := files.Open("secret.txt"); err == nil {
				t.Fatal("non-Go file readable")
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := provider.Resolve(context.Background(), filepath.Join(root, "secret.txt")); !errors.Is(err, ErrTarget) {
			t.Fatalf("non-Go target: %v", err)
		}
		if _, err := provider.Resolve(context.Background(), filepath.Join(root, "missing.go")); err == nil {
			t.Fatal("missing accepted")
		}
		if _, err := provider.Resolve(context.Background(), filepath.Join(root, ignoredDir)); !errors.Is(err, fs.ErrPermission) {
			t.Fatalf("excluded directory entrance: %v", err)
		}
		if _, err := provider.Resolve(context.Background(), filepath.Join(root, ignoredDir, "ignored.go")); !errors.Is(err, fs.ErrPermission) {
			t.Fatalf("excluded source ancestor: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (Provider{URL: server.URL, Token: token}).Resolve(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := (Provider{URL: server.URL, Token: strings.Repeat("b", 48)}).Resolve(context.Background(), root); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("authentication must report reader failure: %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/health", nil)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal("health did not require authentication")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err == nil {
		if _, _, close, err := Open(link); err == nil {
			close()
			t.Fatal("link target accepted")
		}
	}
}
