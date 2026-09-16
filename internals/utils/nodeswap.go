package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func InstallNodeArchive(ctx context.Context, archivePath, filename string, report progressFunc) (installDir string, err error) {
	if report == nil {
		report = func(DownloadProgressMsg) {}
	}

	report(DownloadProgressMsg{Phase: PhaseExtracting, File: filename})

	extractDir := filepath.Join(SwapFilesDir(), "extract_"+fmt.Sprint(time.Now().UnixNano()))

	defer func() {
		if err != nil {
			os.RemoveAll(extractDir)
		}
	}()

	if err = extractArchive(ctx, archivePath, extractDir); err != nil {
		return "", fmt.Errorf("extracting archive: %w", err)
	}

	srcRoot := extractDir
	if entries, _ := os.ReadDir(extractDir); len(entries) == 1 && entries[0].IsDir() {
		srcRoot = filepath.Join(extractDir, entries[0].Name())
	}

	marker := "node.exe"
	if runtime.GOOS != "windows" {
		marker = filepath.Join("bin", "node")
	}
	if _, statErr := os.Stat(filepath.Join(srcRoot, marker)); statErr != nil {
		err = fmt.Errorf("archive layout unexpected: no %s found in %s", marker, srcRoot)
		return "", err
	}

	report(DownloadProgressMsg{Phase: PhaseSwapping, File: filename})

	installDir = filepath.Join(SwapFilesDir(), "node")
	backupDir := filepath.Join(SwapFilesDir(), "node.bak")

	os.RemoveAll(backupDir)

	if _, statErr := os.Stat(installDir); statErr == nil {
		if err = os.Rename(installDir, backupDir); err != nil {
			return "", fmt.Errorf("backing up current toolchain: %w", err)
		}
	}

	if err = os.Rename(srcRoot, installDir); err != nil {

		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, installDir)
		}
		return "", fmt.Errorf("activating new toolchain: %w", err)
	}

	if runtime.GOOS == "windows" {
		if err = ensureWindowsPath(installDir); err != nil {

			os.RemoveAll(installDir)
			if _, statErr := os.Stat(backupDir); statErr == nil {
				_ = os.Rename(backupDir, installDir)
			}
			return "", err
		}
	}

	os.RemoveAll(backupDir)
	return installDir, nil
}
