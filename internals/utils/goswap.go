package utils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// swapFilesDir is where downloaded archives land.
const swapFilesDir = "swap_files"

// SwapFilesDir returns the directory where downloaded archives are stored,
// next to the OpenVM executable.
func SwapFilesDir() string {
	exe, err := os.Executable()
	if err != nil {
		return swapFilesDir
	}
	return filepath.Join(filepath.Dir(exe), swapFilesDir)
}

// progressFunc receives pipeline progress updates; it must not block for long.
type progressFunc func(DownloadProgressMsg)

// InstallGoArchive runs the second half of the pipeline: extract the already
// downloaded and checksum-verified archive, then swap it in as the active Go
// toolchain. On Windows the whole extracted tree is renamed into place (the
// go binary is locked while running); on unix only bin/go is replaced, which
// is a cheap atomic rename. The previous installation is backed up and
// restored automatically if the swap fails.
func InstallGoArchive(ctx context.Context, archivePath, filename string, report progressFunc) (installDir string, err error) {
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

	// goX.Y.Z.os-arch/... → .../go/...
	srcRoot := extractDir
	if entries, _ := os.ReadDir(extractDir); len(entries) == 1 && entries[0].IsDir() {
		srcRoot = filepath.Join(extractDir, entries[0].Name())
	}
	if _, statErr := os.Stat(filepath.Join(srcRoot, "bin")); statErr != nil {
		err = fmt.Errorf("archive layout unexpected: no bin/ directory found in %s", srcRoot)
		return "", err
	}

	report(DownloadProgressMsg{Phase: PhaseSwapping, File: filename})

	if runtime.GOOS == "windows" {
		installDir, err = swapWindows(srcRoot)
	} else {
		installDir, err = swapUnix(srcRoot)
	}
	if err != nil {
		return "", err
	}

	os.RemoveAll(extractDir)
	return installDir, nil
}

// extractArchive unpacks .zip (Windows builds) and the .tar.[gz|xz|zst]
// archives used for unix builds.
func extractArchive(ctx context.Context, archivePath, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZip(archivePath, destDir)
	case strings.HasSuffix(archivePath, ".tar.gz"), strings.HasSuffix(archivePath, ".tgz"):
		return extractTarGz(ctx, archivePath, destDir)
	case strings.HasSuffix(archivePath, ".tar.xz"):
		return extractTarXz(ctx, archivePath, destDir)
	case strings.HasSuffix(archivePath, ".tar.zst"), strings.HasSuffix(archivePath, ".tar.zstd"):
		return extractTarZst(ctx, archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive type: %s", filepath.Base(archivePath))
	}
}

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if err := extractZipEntry(f, destDir); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, destDir string) error {
	name := filepath.Clean(f.Name)
	if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
		return fmt.Errorf("illegal path in archive: %s", f.Name)
	}

	target := filepath.Join(destDir, name)

	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func extractTarGz(ctx context.Context, archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	return extractTarStream(ctx, gz, destDir)
}

func extractTarXz(ctx context.Context, archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	xr, err := xz.NewReader(f)
	if err != nil {
		return err
	}

	return extractTarStream(ctx, xr, destDir)
}

func extractTarZst(ctx context.Context, archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	zr, err := zstd.NewReader(f)
	if err != nil {
		return err
	}
	defer zr.Close()

	return extractTarStream(ctx, zr, destDir)
}

// extractTarStream unpacks a tar file read from r into destDir. It is the
// shared implementation behind the .tar.gz / .tar.xz / .tar.zst extractors.
func extractTarStream(ctx context.Context, r io.Reader, destDir string) error {
	tr := tar.NewReader(r)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("extraction cancelled")
		default:
		}

		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		name := filepath.Clean(hdr.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("illegal path in archive: %s", hdr.Name)
		}
		target := filepath.Join(destDir, name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil && !os.IsExist(err) {
				return err
			}
		}
	}
}

// swapWindows renames the entire toolchain tree into place. The running go
// binary locks its own install dir, so OpenVM's copy lives elsewhere; the
// old tree is moved aside first and restored if anything fails.
func swapWindows(srcRoot string) (string, error) {
	installDir := filepath.Join(SwapFilesDir(), "go")
	backupDir := filepath.Join(SwapFilesDir(), "go.bak")

	os.RemoveAll(backupDir)

	if _, err := os.Stat(installDir); err == nil {
		if err := os.Rename(installDir, backupDir); err != nil {
			return "", fmt.Errorf("backing up current toolchain: %w", err)
		}
	}

	if err := os.Rename(srcRoot, installDir); err != nil {
		// Rollback.
		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, installDir)
		}
		return "", fmt.Errorf("activating new toolchain: %w", err)
	}

	if err := ensureWindowsPath(filepath.Join(installDir, "bin")); err != nil {
		// Rollback.
		os.RemoveAll(installDir)
		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, installDir)
		}
		return "", err
	}

	os.RemoveAll(backupDir)
	return installDir, nil
}

// swapUnix swaps the entire toolchain tree into place, mirroring the
// Windows approach: a new go binary must run against its matching stdlib,
// so replacing only bin/go would mix versions. The old tree is renamed
// aside first and restored if anything fails.
func swapUnix(srcRoot string) (string, error) {
	installDir := currentGoRoot()
	if installDir == "" {
		installDir = filepath.Join(SwapFilesDir(), "go")
	}

	backupDir := installDir + ".bak"
	os.RemoveAll(backupDir)

	if _, err := os.Stat(installDir); err == nil {
		if err := os.Rename(installDir, backupDir); err != nil {
			return "", fmt.Errorf("backing up current toolchain (%s): %w", installDir, err)
		}
	}

	if err := os.Rename(srcRoot, installDir); err != nil {
		// Rollback.
		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, installDir)
		}
		return "", fmt.Errorf("activating new toolchain: %w", err)
	}

	os.RemoveAll(backupDir)
	return installDir, nil
}

// currentGoRoot returns the directory of the go binary currently on PATH, if
// any. Extracting it from `go env GOROOT` would report the GOROOT baked into
// the binary, which is not necessarily where the binary lives.
func currentGoRoot() string {
	path, err := exec.LookPath("go")
	if err != nil {
		return ""
	}
	return filepath.Dir(filepath.Dir(path))
}

func ensureWindowsPath(dir string) error {
	// Prepend rather than append so the managed toolchain wins over any
	// pre-existing Go entry already on the user's PATH.
	script := fmt.Sprintf(
		`$p=[Environment]::GetEnvironmentVariable('Path','User'); if($p -notlike '*%s*'){[Environment]::SetEnvironmentVariable('Path', '%s;'+$p, 'User')}`,
		dir, dir,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adding %s to user PATH: %w: %s", dir, err, strings.TrimSpace(string(out)))
	}
	return nil
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
