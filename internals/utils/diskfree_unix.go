//go:build !windows

package utils

import (
	"os"
	"path/filepath"
	"syscall"
)

func availableSpaceBytes() int64 {
	exe, err := os.Executable()
	if err != nil {
		return 0
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(filepath.Dir(exe), &st); err != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize)
}
