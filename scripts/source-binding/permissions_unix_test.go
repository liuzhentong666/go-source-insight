//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExistingBroadPermissionsRejectedWithoutChanges(t *testing.T) {
	root := t.TempDir()
	source := fixtureDir(t, root, "source")
	state := filepath.Join(root, "state")
	if err := os.Mkdir(state, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(state, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "unrelated"), []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := configure(17, 29, source, state, nil, false); err == nil {
		t.Fatal("Broad pre-existing state directory accepted")
	}
	if info, _ := os.Stat(state); info.Mode().Perm() != 0755 {
		t.Fatal("Pre-existing directory permissions were changed")
	}
	if data, _ := os.ReadFile(filepath.Join(state, "unrelated")); string(data) != "preserve" {
		t.Fatal("Unrelated state data changed")
	}
}
func TestUnreadableHostDirectoryRejected(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Run as non-root to verify unreadable directories")
	}
	root := t.TempDir()
	source := fixtureDir(t, root, "source")
	if err := os.Chmod(source, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(source, 0700)
	if _, err := canonicalDirectory(source); err == nil {
		t.Fatal("Unreadable host directory accepted")
	}
}

func TestExistingDifferentOwnerRejectedWithoutChanges(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Changing fixture ownership requires root; exercised separately")
	}
	root := t.TempDir()
	source := fixtureDir(t, root, "source")
	state := filepath.Join(root, "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(state, "unrelated")
	if err := os.WriteFile(marker, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(state, 10001, 10001); err != nil {
		t.Fatal(err)
	}
	if err := configure(17, 29, source, state, nil, false); err == nil {
		t.Fatal("Different directory owner accepted")
	}
	if data, _ := os.ReadFile(marker); string(data) != "preserve" {
		t.Fatal("Unrelated state data changed")
	}
	if info, _ := os.Stat(state); info.Mode().Perm() != 0700 {
		t.Fatal("Existing permissions changed")
	}
}
