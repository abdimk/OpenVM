package api

import (
	"testing"
)

func TestParseDockerListing(t *testing.T) {
	body := `<a href="docker-28.9.0.tgz">old</a>
<a href='docker-28.10.0.tgz'>new</a>
<a href="docker-28.10.0.tgz">duplicate</a>
<a href="docker-rootless-extras-29.0.0.tgz">extras</a>
<a href="docker-29.0.0-rc1.tgz">preview</a>
<a href="../docker-29.0.0.tgz">parent</a>
<a href="https://example.com/docker-29.0.0.tgz">external</a>
<a href="docker-29.0.0.zip">wrong platform</a>`
	base := "https://download.docker.com/linux/static/stable/x86_64/"
	releases := parseDockerListing(body, base, "tgz", "linux", "amd64")
	if len(releases) != 2 {
		t.Fatalf("got %d releases, want 2", len(releases))
	}
	for i, version := range []string{"28.10.0", "28.9.0"} {
		r := releases[i]
		if r.Version != version || !r.Stable || len(r.Files) != 1 {
			t.Fatalf("unexpected release: %+v", r)
		}
		f := r.Files[0]
		if f.Filename != "docker-"+version+".tgz" || f.URL != base+f.Filename || f.OS != "linux" || f.Arch != "amd64" || f.Kind != "archive" || f.Version != version {
			t.Errorf("unexpected artifact: %+v", f)
		}
	}
}

func TestDockerTarget(t *testing.T) {
	for _, tc := range []struct{ os, arch, path, ext string }{
		{"linux", "amd64", "linux/static/stable/x86_64/", "tgz"},
		{"linux", "arm64", "linux/static/stable/aarch64/", "tgz"},
		{"darwin", "arm64", "mac/static/stable/aarch64/", "tgz"},
		{"darwin", "amd64", "mac/static/stable/x86_64/", "tgz"},
		{"windows", "amd64", "win/static/stable/x86_64/", "zip"},
	} {
		base, ext, err := dockerTarget(tc.os, tc.arch)
		if err != nil || base != dockerDownloadBase+"/"+tc.path || ext != tc.ext {
			t.Errorf("%s/%s: %q, %q, %v", tc.os, tc.arch, base, ext, err)
		}
	}
	if _, _, err := dockerTarget("windows", "arm64"); err == nil {
		t.Fatal("expected unsupported platform error")
	}
}
