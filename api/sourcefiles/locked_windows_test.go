//go:build windows

package sourcefiles

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"go-ai-study/api/fault"
)

func TestWorkspaceFailsWhenWindowsSourceIsUnreadable(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "7", "1")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(project, "locked.go")
	if err := os.WriteFile(file, []byte("package sample\n"), 0600); err != nil {
		t.Fatal(err)
	}
	name, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	w, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Scan(context.Background(), 7, 1, "."); !errors.Is(err, fault.ErrSourceUnavailable) {
		t.Fatalf("unreadable scan=%v", err)
	}
	if _, err := w.Analyze(context.Background(), 7, 1, "."); !errors.Is(err, fault.ErrSourceUnavailable) {
		t.Fatalf("unreadable analysis=%v", err)
	}
}
