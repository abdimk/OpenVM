package swap

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

type testTarEntry struct {
	hdr  tar.Header
	data []byte
}

func writeTarGz(t *testing.T, entries []testTarEntry) string {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, e := range entries {
		if err := tw.WriteHeader(&e.hdr); err != nil {
			t.Fatalf("WriteHeader(%s): %v", e.hdr.Name, err)
		}
		if _, err := tw.Write(e.data); err != nil {
			t.Fatalf("Write(%s): %v", e.hdr.Name, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	p := filepath.Join(t.TempDir(), "test.tar.gz")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	return p
}

func TestExtractTarSymlinkBeforeTarget(t *testing.T) {
	archive := writeTarGz(t, []testTarEntry{
		{
			hdr: tar.Header{
				Typeflag: tar.TypeSymlink,
				Name:     "python-3.14/bin/idle3.14",
				Linkname: "idle3",
				Mode:     0o777,
			},
		},
		{
			hdr: tar.Header{
				Typeflag: tar.TypeReg,
				Name:     "python-3.14/bin/idle3",
				Mode:     0o755,
				Size:     int64(len([]byte("#!/bin/sh\nidle\n"))),
			},
			data: []byte("#!/bin/sh\nidle\n"),
		},
		{
			hdr: tar.Header{
				Typeflag: tar.TypeSymlink,
				Name:     "python-3.14/lib/dangling",
				Linkname: "missing/target",
				Mode:     0o777,
			},
		},
	})

	dest := filepath.Join(t.TempDir(), "out")
	if err := extractTarGz(context.Background(), archive, dest); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	idle := filepath.Join(dest, "python-3.14", "bin", "idle3.14")
	fi, err := os.Stat(idle)
	if err != nil {
		t.Fatalf("idle3.14 was not created: %v", err)
	}
	if fi.IsDir() {
		t.Fatalf("idle3.14 is a directory")
	}

	got, err := os.ReadFile(idle)
	if err != nil {
		t.Fatalf("read idle3.14: %v", err)
	}
	if string(got) != "#!/bin/sh\nidle\n" {
		t.Errorf("idle3.14 content = %q, want the referent content", string(got))
	}
}
