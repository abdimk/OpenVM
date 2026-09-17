package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRustTargetTriple(t *testing.T) {
	cases := []struct {
		os, arch, want string
		ok             bool
	}{
		{"windows", "amd64", "x86_64-pc-windows-msvc", true},
		{"windows", "arm64", "aarch64-pc-windows-msvc", true},
		{"linux", "amd64", "x86_64-unknown-linux-gnu", true},
		{"linux", "arm64", "aarch64-unknown-linux-gnu", true},
		{"darwin", "amd64", "x86_64-apple-darwin", true},
		{"darwin", "arm64", "aarch64-apple-darwin", true},
		{"linux", "arm", "", false},
		{"windows", "386", "", false},
	}
	for _, c := range cases {
		got, ok := rustTargetTriple(c.os, c.arch)
		if got != c.want || ok != c.ok {
			t.Errorf("rustTargetTriple(%s, %s) = (%q, %v), want (%q, %v)", c.os, c.arch, got, ok, c.want, c.ok)
		}
	}
}

const rustManifestSample = `
manifest-version = "2"
date = "2026-09-03"

[manifest]
version = "1.85.1"

[pkg.rust]
version = "1.85.1 (8d68f698c 2026-08-01)"

[pkg.rust.target.x86_64-unknown-linux-gnu]
available = true
url = "https://static.rust-lang.org/dist/2026-09-03/rust-1.85.1-x86_64-unknown-linux-gnu.tar.gz"
hash = "aaaa"
xz_url = "https://static.rust-lang.org/dist/2026-09-03/rust-1.85.1-x86_64-unknown-linux-gnu.tar.xz"
xz_hash = "bbbb"

[pkg.rust.target.x86_64-unknown-linux-gnu.components]
"rustc-x86_64-unknown-linux-gnu" = { available = true, url = "component", hash = "unused" }

[pkg.rust.target.x86_64-pc-windows-msvc]
available = true
url = "https://static.rust-lang.org/dist/2026-09-03/rust-1.85.1-x86_64-pc-windows-msvc.tar.gz"
hash = "cccc"
xz_url = "https://static.rust-lang.org/dist/2026-09-03/rust-1.85.1-x86_64-pc-windows-msvc.tar.xz"
xz_hash = "dddd"
`

func TestParseRustManifest(t *testing.T) {
	m, err := parseRustManifest([]byte(rustManifestSample), "x86_64-unknown-linux-gnu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.version != "1.85.1" {
		t.Errorf("version = %q, want 1.85.1", m.version)
	}
	if !strings.HasSuffix(m.url, ".tar.xz") {
		t.Errorf("url = %q, want the .tar.xz preference", m.url)
	}
	if m.sha256 != "bbbb" {
		t.Errorf("sha256 = %q, want bbbb (xz_hash paired with xz_url)", m.sha256)
	}

	m2, err := parseRustManifest([]byte(rustManifestSample), "x86_64-pc-windows-msvc")
	if err != nil {
		t.Fatalf("unexpected error parsing windows target: %v", err)
	}
	if m2.sha256 != "dddd" {
		t.Errorf("sha256 = %q, want dddd", m2.sha256)
	}
}

func TestParseRustManifestMissingTarget(t *testing.T) {
	_, err := parseRustManifest([]byte(rustManifestSample), "aarch64-apple-darwin")
	if err == nil {
		t.Fatal("expected error when the target section is absent")
	}
}

func TestFetchRustArtifactHTTP(t *testing.T) {
	oldURL := rustChannelURL
	rustChannelURL = ""
	defer func() { rustChannelURL = oldURL }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(rustManifestSample))
	}))
	defer srv.Close()
	rustChannelURL = srv.URL

	file, err := fetchRustArtifactForTriple("x86_64-unknown-linux-gnu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file.Version != "1.85.1" {
		t.Errorf("version = %q, want 1.85.1", file.Version)
	}
	if file.Filename != "rust-1.85.1-x86_64-unknown-linux-gnu.tar.xz" {
		t.Errorf("filename = %q", file.Filename)
	}
	if file.Kind != "archive" {
		t.Errorf("kind = %q, want archive", file.Kind)
	}
}
