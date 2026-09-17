package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func InstallKubectlBinary(ctx context.Context, binaryPath, filename string, report progressFunc) (installDir string, err error) {
	if report == nil {
		report = func(DownloadProgressMsg) {}
	}

	report(DownloadProgressMsg{Phase: PhaseSwapping, File: filename})

	installDir = filepath.Join(SwapFilesDir(), "kubectl")
	backupDir := filepath.Join(SwapFilesDir(), "kubectl.bak")

	os.RemoveAll(backupDir)

	if _, statErr := os.Stat(installDir); statErr == nil {
		if err = os.Rename(installDir, backupDir); err != nil {
			return "", fmt.Errorf("backing up current toolchain: %w", err)
		}
	}

	if err = os.MkdirAll(installDir, 0o755); err != nil {
		restoreSwapDir(backupDir, installDir)
		return "", err
	}

	if err = os.Rename(binaryPath, filepath.Join(installDir, filename)); err != nil {
		os.RemoveAll(installDir)
		restoreSwapDir(backupDir, installDir)
		return "", fmt.Errorf("activating kubectl: %w", err)
	}

	if err = os.Chmod(filepath.Join(installDir, filename), 0o755); err != nil {
		os.RemoveAll(installDir)
		restoreSwapDir(backupDir, installDir)
		return "", err
	}

	if err = activatePath(installDir); err != nil {
		os.RemoveAll(installDir)
		restoreSwapDir(backupDir, installDir)
		return "", err
	}

	os.RemoveAll(backupDir)
	return installDir, nil
}

func restoreSwapDir(backupDir, installDir string) {
	if _, statErr := os.Stat(backupDir); statErr == nil {
		_ = os.Rename(backupDir, installDir)
	}
}
