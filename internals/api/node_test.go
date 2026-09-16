package api

import "testing"

func TestNodeReleaseInfosFromReleases(t *testing.T) {
	releases := []nodeRelease{
		{Version: "v26.8.2", Date: "2026-09-08"},
		{Version: "v27.0.0-rc.1", Date: "2026-09-10"},
		{Version: "v22.23.0", Date: "2026-09-03", LTS: "Jod"},
		{Version: "v22.23.0-rc.0"},
		{Version: "v18.0.0", Date: "2022-04-19", LTS: false},
		{Version: "v17.9.1"},
		{Version: "v4.9.1"},
	}

	got := nodeReleaseInfosFromReleases(releases)

	want := []NodeReleaseInfo{
		{Version: "26.8.2", Date: "2026-09-08"},
		{Version: "22.23.0", Date: "2026-09-03", LTS: "Jod"},
		{Version: "18.0.0", Date: "2022-04-19"},
	}
	if len(got) != len(want) {
		t.Fatalf("nodeReleaseInfosFromReleases = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestNodeMajorSupported(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"v26.8.2", true},
		{"v18.0.0", true},
		{"v17.9.1", false},
		{"v4.9.1", false},
		{"notaversion", false},
	}
	for _, c := range cases {
		if got := nodeMajorSupported(c.version); got != c.want {
			t.Errorf("nodeMajorSupported(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}

func TestParseNodeSHASUMS(t *testing.T) {
	fixture := `b7f9a8b6cd2f0b6d  node-v22.23.0-win-x64.zip
9c1d5a3f  *node-v22.23.0-linux-x64.tar.xz
deadbeef  node-v22.23.0-darwin-arm64.tar.gz

garbage-line-without-two-fields
`
	got := parseNodeSHASUMS(fixture)

	if want := "b7f9a8b6cd2f0b6d"; got["node-v22.23.0-win-x64.zip"] != want {
		t.Errorf("win-x64 sha = %q, want %q", got["node-v22.23.0-win-x64.zip"], want)
	}
	if want := "9c1d5a3f"; got["node-v22.23.0-linux-x64.tar.xz"] != want {
		t.Errorf("linux-x64 sha (star prefix) = %q, want %q", got["node-v22.23.0-linux-x64.tar.xz"], want)
	}
	if want := "deadbeef"; got["node-v22.23.0-darwin-arm64.tar.gz"] != want {
		t.Errorf("darwin-arm64 sha = %q, want %q", got["node-v22.23.0-darwin-arm64.tar.gz"], want)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 parsed entries, got %d: %v", len(got), got)
	}
}

func TestNodeArtifactTarget(t *testing.T) {
	cases := []struct {
		goos, goarch string
		wantToken    string
		wantExts     int
		wantOK       bool
	}{
		{"windows", "amd64", "win-x64", 1, true},
		{"windows", "arm64", "win-arm64", 1, true},
		{"windows", "386", "", 0, false},
		{"darwin", "arm64", "darwin-arm64", 1, true},
		{"linux", "amd64", "linux-x64", 2, true},
		{"linux", "arm64", "linux-arm64", 2, true},
		{"freebsd", "amd64", "", 0, false},
	}
	for _, c := range cases {
		token, exts, ok := nodeArtifactTarget(c.goos, c.goarch)
		if ok != c.wantOK || token != c.wantToken || len(exts) != c.wantExts {
			t.Errorf("nodeArtifactTarget(%q, %q) = (%q, %v, %v), want (%q, %d entries, %v)",
				c.goos, c.goarch, token, exts, ok, c.wantToken, c.wantExts, c.wantOK)
		}
	}
}

func TestNodeArtifactForMachine(t *testing.T) {
	file, ok := nodeArtifactForMachine("v22.23.0", "windows", "amd64")
	if ok {
		if want := "node-v22.23.0-win-x64.zip"; file.Filename != want {
			t.Errorf("filename = %q, want %q", file.Filename, want)
		}
		if file.SHA256 == "" {
			t.Error("sha256 must come from SHASUMS256.txt, got empty")
		}
		if want := "https://nodejs.org/dist/v22.23.0/node-v22.23.0-win-x64.zip"; file.URL != want {
			t.Errorf("url = %q, want %q", file.URL, want)
		}
		if file.Kind != "archive" || file.OS != "windows" || file.Arch != "amd64" {
			t.Errorf("got %s/%s kind=%s, want windows/amd64 archive", file.OS, file.Arch, file.Kind)
		}
	}

	if _, ok := nodeArtifactForMachine("v22.23.0", "plan9", "amd64"); ok {
		t.Error("expected no artifact for unsupported OS, got ok=true")
	}
}
