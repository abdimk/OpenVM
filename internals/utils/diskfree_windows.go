package utils

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func availableSpaceBytes() int64 {
	exe, err := os.Executable()
	if err != nil {
		return 0
	}
	path, err := windows.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return 0
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(path, &free, &total, &totalFree); err != nil {
		return 0
	}
	return int64(free)
}
