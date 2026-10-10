package source

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestScanRejectsUnreadableSources(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX file permissions")
	}
	for _, directory := range []bool{false, true} {
		name := "file"
		if directory {
			name = "directory"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			blocked := filepath.Join(root, "blocked.go")
			if directory {
				blocked = filepath.Join(root, "blocked")
				if err := os.Mkdir(blocked, 0700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(blocked, []byte("package sample\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(blocked, 0000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(blocked, 0700) })
			var probe error
			if directory {
				_, probe = os.ReadDir(blocked)
			} else {
				_, probe = os.ReadFile(blocked)
			}
			if probe == nil {
				t.Skip("requires non-privileged user")
			}
			if _, err := Scan(context.Background(), root); !errors.Is(err, fs.ErrPermission) {
				t.Fatalf("error = %v, want permission failure", err)
			}
		})
	}
}
