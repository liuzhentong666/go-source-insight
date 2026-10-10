//go:build !windows

package main

import (
	"errors"
	"os"
	"syscall"
)

func isUnsafeLink(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }
func restrictDirectory(directory string, newlyCreated bool) error {
	if !newlyCreated {
		info, err := os.Stat(directory)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
			return errors.New("Existing configuration directory is not owner-only. Use a new state directory or restrict it deliberately; existing permissions were not rewritten.")
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uint32(os.Geteuid()) {
			return errors.New("Existing configuration directory is owned by another user; permissions and binding files were not changed.")
		}
		return nil
	}
	if os.Chmod(directory, 0700) != nil {
		return errors.New("Cannot restrict configuration-directory permissions.")
	}
	return nil
}
