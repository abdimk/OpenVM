package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// InstallClangArchive runs the second half of the Clang/LLVM pipeline:
// extract the already downloaded archive, then swap it in as the active LLVM
// toolchain. The whole tree lives under SwapFilesDir()/llvm and its bin/
// directory is registered on the user PATH on Windows. The previous
// installation is backed up and restored automatically if the swap fails.
func InstallClangArchive(ctx context.Context, archivePath, filename string, report progressFunc) (installDir string, err error) {
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

	// LLVM-23.1.1-Linux-X64/bin and clang+llvm-23.1.1-x86_64-.../bin are the
	// two layouts shipped; the bin/ directory sits either at the root of the
	// archive or inside a single top-level folder.
	srcRoot := extractDir
	if entries, _ := os.ReadDir(extractDir); len(entries) == 1 && entries[0].IsDir() {
		srcRoot = filepath.Join(extractDir, entries[0].Name())
	}
	if _, statErr := os.Stat(filepath.Join(srcRoot, "bin")); statErr != nil {
		err = fmt.Errorf("archive layout unexpected: no bin/ directory found in %s", srcRoot)
		return "", err
	}

	report(DownloadProgressMsg{Phase: PhaseSwapping, File: filename})

	installDir = filepath.Join(SwapFilesDir(), "llvm")
	backupDir := filepath.Join(SwapFilesDir(), "llvm.bak")

	os.RemoveAll(backupDir)

	if _, statErr := os.Stat(installDir); statErr == nil {
		if err = os.Rename(installDir, backupDir); err != nil {
			return "", fmt.Errorf("backing up current toolchain: %w", err)
		}
	}

	if err = os.Rename(srcRoot, installDir); err != nil {
		// Rollback.
		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, installDir)
		}
		return "", fmt.Errorf("activating new toolchain: %w", err)
	}

	if runtime.GOOS == "windows" {
		if err = ensureWindowsPath(filepath.Join(installDir, "bin")); err != nil {
			// Rollback.
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