package main

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestWindowsDeniedReplacementRetainsCommittedMapping(t *testing.T) {
	root := t.TempDir()
	source := fixtureDir(t, root, "source")
	state := filepath.Join(root, "state")
	if err := configure(17, 29, source, state, nil, false); err != nil {
		t.Fatal(err)
	}
	_, before, generation := committed(t, state)
	file := filepath.Join(state, "source-bindings.compose.json")
	encoded, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(encoded, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	if err = configure(31, 47, source, state, nil, false); err == nil {
		t.Fatal("Windows replacement succeeded despite denied delete sharing")
	}
	_, after, active := committed(t, state)
	if !bytes.Equal(before, after) || active != generation {
		t.Fatal("Denied Windows replacement changed committed mapping")
	}
	if _, err := os.Stat(generation); err != nil {
		t.Fatal("Old generation disappeared")
	}
}
