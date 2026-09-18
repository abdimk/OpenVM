package swap

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

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

func extractTarStream(ctx context.Context, r io.Reader, destDir string) error {
	tr := tar.NewReader(r)

	var symlinks []tarSymlinkEntry

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("extraction cancelled")
		default:
		}

		hdr, err := tr.Next()
		if err == io.EOF {
			break
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
			symlinks = append(symlinks, tarSymlinkEntry{target: target, linkname: hdr.Linkname})
		}
	}

	for _, s := range symlinks {
		if err := createSymlink(s.target, s.linkname); err != nil {
			return err
		}
	}

	return nil
}

type tarSymlinkEntry struct {
	target   string
	linkname string
}

func createSymlink(target, linkname string) error {
	_ = os.Remove(target)
	if err := os.Symlink(linkname, target); err == nil || os.IsExist(err) {
		return nil
	}
	return copySymlinkReferent(target, linkname)
}

func copySymlinkReferent(target, linkname string) error {
	ref := linkname
	if !filepath.IsAbs(ref) {
		ref = filepath.Join(filepath.Dir(target), ref)
	}
	ref = filepath.Clean(ref)

	fi, err := os.Stat(ref)
	if err != nil || fi.IsDir() {
		return nil
	}

	in, err := os.Open(ref)
	if err != nil {
		return nil
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fi.Mode())
	if err != nil {
		return nil
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
