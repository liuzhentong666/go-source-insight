//go:build windows

package localfiles

import (
	"io/fs"
	"syscall"
)

func unsafe(info fs.FileInfo) bool {
	attrs, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return info.Mode()&fs.ModeSymlink != 0 || (ok && attrs.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0)
}
