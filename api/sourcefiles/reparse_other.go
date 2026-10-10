//go:build !windows

package sourcefiles

import "io/fs"

func isReparse(info fs.FileInfo) bool { return info.Mode()&fs.ModeSymlink != 0 }
