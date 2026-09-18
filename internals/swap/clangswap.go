package swap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/abdimk/openvm/internals/utils"
)

func InstallClangArchive(ctx context.Context, archivePath, filename string, report progressFunc) (installDir string, err error) {
	if report == nil {
		report = func(utils.DownloadProgressMsg) {}
	}

	report(utils.DownloadProgressMsg{Phase: utils.PhaseExtracting, File: filename})

	extractDir := filepath.Join(utils.SwapFilesDir(), "extract_"+fmt.Sprint(time.Now().UnixNano()))

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
	if _, statErr := os.Stat(filepath.Join(srcRoot, "bin")); statErr != nil {
		err = fmt.Errorf("archive layout unexpected: no bin/ directory found in %s", srcRoot)
		return "", err
	}

	report(utils.DownloadProgressMsg{Phase: utils.PhaseSwapping, File: filename})

	installDir = filepath.Join(utils.SwapFilesDir(), "llvm")
	backupDir := filepath.Join(utils.SwapFilesDir(), "llvm.bak")

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

	if err = utils.ActivatePath(filepath.Join(installDir, "bin")); err != nil {

		os.RemoveAll(installDir)
		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, installDir)
		}
		return "", err
	}

	os.RemoveAll(backupDir)
	return installDir, nil
}
