package sourcefiles

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"go-ai-study/api/fault"
)

func makeDirectoryLink(t *testing.T, name, target string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		if err := os.Symlink(target, name); err != nil {
			t.Fatal(err)
		}
		return
	}
	// Junctions need no developer-mode symlink privilege and exercise Windows
	// reparse-point handling independently of ModeSymlink.
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference = 'Stop'; New-Item -ItemType Junction -Path $env:TEST_SOURCE_LINK -Target $env:TEST_SOURCE_TARGET | Out-Null")
	cmd.Env = append(os.Environ(), "TEST_SOURCE_LINK="+name, "TEST_SOURCE_TARGET="+target)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create test junction: %v %s", err, output)
	}
}

func TestWorkspaceRejectsLinksAtEveryBindingLevel(t *testing.T) {
	for _, level := range []string{"source root", "owner", "project", "target directory", "target ancestor"} {
		t.Run(level, func(t *testing.T) {
			base := t.TempDir()
			outside := filepath.Join(base, "outside")
			sources := filepath.Join(base, "sources")
			if err := os.Mkdir(outside, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(outside, "outside.go"), []byte("package outside"), 0600); err != nil {
				t.Fatal(err)
			}
			target := "."
			var link string
			switch level {
			case "source root":
				link = sources
			case "owner":
				if err := os.Mkdir(sources, 0700); err != nil {
					t.Fatal(err)
				}
				link = filepath.Join(sources, "7")
			case "project":
				if err := os.MkdirAll(filepath.Join(sources, "7"), 0700); err != nil {
					t.Fatal(err)
				}
				link = filepath.Join(sources, "7", "1")
			case "target directory", "target ancestor":
				if err := os.MkdirAll(filepath.Join(sources, "7", "1"), 0700); err != nil {
					t.Fatal(err)
				}
				link = filepath.Join(sources, "7", "1", "linked")
				target = "linked"
				if level == "target ancestor" {
					target = "linked/outside.go"
				}
			}
			makeDirectoryLink(t, link, outside)
			w, err := New(sources)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.Check(7, 1, target); !errors.Is(err, fault.ErrUnsafePath) {
				t.Fatalf("linked check allowed: %v", err)
			}
			if _, err := w.Scan(context.Background(), 7, 1, target); !errors.Is(err, fault.ErrUnsafePath) {
				t.Fatalf("linked scan allowed: %v", err)
			}
			if _, err := w.Analyze(context.Background(), 7, 1, target); !errors.Is(err, fault.ErrUnsafePath) {
				t.Fatalf("linked analysis allowed: %v", err)
			}
		})
	}
}

func TestWorkspaceRejectsFileLinkAndForeignSources(t *testing.T) {
	sources := t.TempDir()
	project := filepath.Join(sources, "7", "1")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "foreign.go")
	if err := os.WriteFile(outside, []byte("package foreign"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(project, "linked.go")
	if err := os.Symlink(outside, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("file symlink requires developer mode; directory junction tests still run")
		}
		t.Fatal(err)
	}
	w, err := New(sources)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Scan(context.Background(), 7, 1, "linked.go"); !errors.Is(err, fault.ErrUnsafePath) {
		t.Fatalf("foreign source link accepted: %v", err)
	}
}

func TestWorkspaceMissingSourcesFailAndTargetsArePortable(t *testing.T) {
	w, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Scan(context.Background(), 7, 1, "."); !errors.Is(err, fault.ErrSourceUnavailable) {
		t.Fatalf("missing project scan=%v", err)
	}
	if _, err := w.Analyze(context.Background(), 7, 1, "."); !errors.Is(err, fault.ErrSourceUnavailable) {
		t.Fatalf("missing project analysis=%v", err)
	}
	for _, bad := range []string{"../other", "/other", "C:/other", "\\\\host/share", "file.go:stream", "aux.go", "COM1", "LPT9.log", "pkg/.", "pkg//file.go", "file.go.", "file.go "} {
		if _, err := NormalizeTarget(bad); !errors.Is(err, fault.ErrInvalidTarget) {
			t.Fatalf("unsafe portable target accepted: %q", bad)
		}
	}
}
