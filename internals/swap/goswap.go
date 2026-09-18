package swap

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/abdimk/openvm/internals/utils"
)

type progressFunc func(utils.DownloadProgressMsg)

func InstallGoArchive(ctx context.Context, archivePath, filename string, report progressFunc) (installDir string, err error) {
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

	if runtime.GOOS == "windows" {
		installDir, err = swapWindows(srcRoot)
	} else {
		installDir, err = swapUnix(srcRoot)
	}
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		if err = utils.ActivatePath(filepath.Join(installDir, "bin")); err != nil {
			return "", err
		}
	}

	os.RemoveAll(extractDir)
	return installDir, nil
}

func swapWindows(srcRoot string) (string, error) {
	installDir := filepath.Join(utils.SwapFilesDir(), "go")
	backupDir := filepath.Join(utils.SwapFilesDir(), "go.bak")

	os.RemoveAll(backupDir)

	if _, err := os.Stat(installDir); err == nil {
		if err := os.Rename(installDir, backupDir); err != nil {
			return "", fmt.Errorf("backing up current toolchain: %w", err)
		}
	}

	if err := os.Rename(srcRoot, installDir); err != nil {

		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, installDir)
		}
		return "", fmt.Errorf("activating new toolchain: %w", err)
	}

	if err := utils.EnsureWindowsPath(filepath.Join(installDir, "bin")); err != nil {

		os.RemoveAll(installDir)
		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, installDir)
		}
		return "", err
	}
	utils.PrependProcessPath(filepath.Join(installDir, "bin"))

	os.RemoveAll(backupDir)
	return installDir, nil
}

func swapUnix(srcRoot string) (string, error) {
	installDir := currentGoRoot()
	if installDir == "" {
		installDir = filepath.Join(utils.SwapFilesDir(), "go")
	}

	backupDir := installDir + ".bak"
	os.RemoveAll(backupDir)

	if _, err := os.Stat(installDir); err == nil {
		if err := os.Rename(installDir, backupDir); err != nil {
			return "", fmt.Errorf("backing up current toolchain (%s): %w", installDir, err)
		}
	}

	if err := os.Rename(srcRoot, installDir); err != nil {

		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, installDir)
		}
		return "", fmt.Errorf("activating new toolchain: %w", err)
	}

	os.RemoveAll(backupDir)
	return installDir, nil
}

func currentGoRoot() string {
	path, err := exec.LookPath("go")
	if err != nil {
		return ""
	}
	return filepath.Dir(filepath.Dir(path))
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func copyDirContents(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(srcDir, e.Name()), filepath.Join(dstDir, e.Name()), 0o755); err != nil {
			return err
		}
	}
	return nil
}
