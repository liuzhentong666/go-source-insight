//go:build windows

package sourcefiles

import (
	"io/fs"
	"syscall"
)

func isReparse(info fs.FileInfo) bool {
	if info.Mode()&fs.ModeSymlink != 0 {
		return true
	}
	attrs, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && attrs.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
