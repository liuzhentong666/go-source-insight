//go:build !windows

package localfiles

import "io/fs"

func unsafe(info fs.FileInfo) bool { return info.Mode()&fs.ModeSymlink != 0 }
