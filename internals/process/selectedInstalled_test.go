package process

import (
	"testing"

	"github.com/abdimk/openvm/internals/api"
	"github.com/abdimk/openvm/internals/utils"
)

func TestExtractPythonVersion(t *testing.T) {
	cases := []struct {
		output, want string
	}{
		{"Python 3.14.7", "3.14.7"},
		{"Python 3.11.0", "3.11.0"},
		{"unknown", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := extractPythonVersion(c.output); got != c.want {
			t.Errorf("extractPythonVersion(%q) = %q, want %q", c.output, got, c.want)
		}
	}
}

func TestExtractClangVersion(t *testing.T) {
	for _, out := range []string{"clang version 20.1.8", "Ubuntu clang version 16.0.6"} {
		if got := extractClangVersion(out); got == "" {
			t.Errorf("extractClangVersion(%q) returned empty", out)
		}
	}
}

func TestExtractNodeVersion(t *testing.T) {
	cases := []struct {
		output, want string
	}{
		{"v22.23.0", "22.23.0"},
		{"v26.8.2", "26.8.2"},
		{"node v26.8.2", "26.8.2"},
		{"unknown", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := extractNodeVersion(c.output); got != c.want {
			t.Errorf("extractNodeVersion(%q) = %q, want %q", c.output, got, c.want)
		}
	}
}

func TestBuildVersionEntriesKeepsLazyReleases(t *testing.T) {
	releases := []api.Release{
		{Version: "26.8.2", Stable: true},
		{Version: "22.23.0", Stable: true},
	}

	entries := buildVersionEntries(utils.Language{Name: utils.Node}, "", releases)
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2 (lazy releases must not be dropped)", len(entries))
	}
	if entries[0].title != "26.8.2" || entries[1].title != "22.23.0" {
		t.Errorf("titles = [%q, %q], want newest first", entries[0].title, entries[1].title)
	}
	if entries[0].file.Filename != "" {
		t.Errorf("lazy entry should have no file yet, got %q", entries[0].file.Filename)
	}
}

func TestBuildVersionEntriesPinsCurrentAndPrefillsGoFiles(t *testing.T) {
	releases := []api.Release{
		{Version: "go1.25.2", Stable: true, Files: []api.File{
			{Filename: "go1.25.2.windows-amd64.zip", OS: "windows", Arch: "amd64", Kind: "archive"},
		}},
		{Version: "go1.25.1", Stable: true, Files: []api.File{
			{Filename: "go1.25.1.windows-amd64.zip", OS: "windows", Arch: "amd64", Kind: "archive"},
		}},
	}

	entries := buildVersionEntries(utils.Language{Name: utils.Go, Version: "go version go1.25.1 windows/amd64"}, "", releases)
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].title != "go1.25.1" || !entries[0].current {
		t.Errorf("first entry = %q (current=%v), want go1.25.1 pinned current", entries[0].title, entries[0].current)
	}
	if entries[0].file.Filename == "" {
		t.Error("Go entry should carry its eagerly resolved file")
	}
}

func TestBuildVersionEntriesPinsCurrentDocker(t *testing.T) {
	releases := []api.Release{
		{Version: "29.8.1", Stable: true, Files: []api.File{{Filename: "docker-29.8.1.tgz"}}},
		{Version: "28.4.0", Stable: true, Files: []api.File{{Filename: "docker-28.4.0.tgz"}}},
		{Version: "27.5.1", Stable: true, Files: []api.File{{Filename: "docker-27.5.1.tgz"}}},
	}

	entries := buildVersionEntries(utils.Language{Name: utils.Docker, Version: "Docker version 28.4.0, build d8eb465"}, "", releases)
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
	if entries[0].title != "28.4.0" || !entries[0].current {
		t.Errorf("first entry = %q (current=%v), want 28.4.0 pinned current", entries[0].title, entries[0].current)
	}
	if entries[1].title != "29.8.1" {
		t.Errorf("second entry = %q, want 29.8.1", entries[1].title)
	}
}

func TestBuildVersionEntriesPrependsUnknownCurrentDocker(t *testing.T) {
	releases := []api.Release{
		{Version: "29.8.1", Stable: true, Files: []api.File{{Filename: "docker-29.8.1.tgz"}}},
	}

	entries := buildVersionEntries(utils.Language{Name: utils.Docker, Version: "Docker version 99.0.0, build dummy"}, "", releases)
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].title != "99.0.0" || !entries[0].current {
		t.Errorf("first entry = %q (current=%v), want 99.0.0 pinned current", entries[0].title, entries[0].current)
	}
}

func TestBuildVersionEntriesPinsCurrentKubernetes(t *testing.T) {
	releases := []api.Release{
		{Version: "v1.37.0", Stable: true},
		{Version: "v1.36.4", Stable: true},
		{Version: "v1.9.3", Stable: true},
	}

	entries := buildVersionEntries(utils.Language{Name: utils.Kubernetes, Version: "Client Version: v1.36.4\nKustomize Version: v5.6.0"}, "", releases)
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
	if entries[0].title != "v1.36.4" || !entries[0].current {
		t.Errorf("first entry = %q (current=%v), want v1.36.4 pinned current", entries[0].title, entries[0].current)
	}
	if entries[1].title != "v1.37.0" {
		t.Errorf("second entry = %q, want v1.37.0", entries[1].title)
	}
}

func TestExtractKubernetesVersion(t *testing.T) {
	cases := []struct{ output, want string }{
		{"Client Version: v1.37.0", "v1.37.0"},
		{"Kubernetes v1.36.4", "v1.36.4"},
		{"unknown", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := extractKubernetesVersion(c.output); got != c.want {
			t.Errorf("extractKubernetesVersion(%q) = %q, want %q", c.output, got, c.want)
		}
	}
}
