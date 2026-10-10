//go:build windows

package handlers_test

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceHTTPRejectsWindowsJunctionTarget(t *testing.T) {
	f := newHTTPFixture(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "foreign.go"), []byte("package foreign"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(f.root, "7", "1", "foreign")
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference = 'Stop'; New-Item -ItemType Junction -Path $env:TEST_SOURCE_LINK -Target $env:TEST_SOURCE_TARGET | Out-Null")
	cmd.Env = append(os.Environ(), "TEST_SOURCE_LINK="+link, "TEST_SOURCE_TARGET="+outside)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create test junction: %v %s", err, output)
	}
	for _, route := range []string{"scan", "analyze", "index"} {
		w := f.request(t, http.MethodPost, "/api/v1/projects/1/"+route, `{"target":"foreign/foreign.go"}`, 7)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "unsafe_path") {
			t.Fatalf("junction HTTP %s=%d %s", route, w.Code, w.Body.String())
		}
	}
	// A root scan also must not report or analyze foreign contents.
	w := f.request(t, http.MethodPost, "/api/v1/projects/1/scan", `{}`, 7)
	if w.Code == http.StatusOK && strings.Contains(w.Body.String(), "foreign.go") {
		t.Fatalf("foreign source traversed: %s", w.Body.String())
	}
}
