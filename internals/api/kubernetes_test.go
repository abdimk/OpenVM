package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchKubernetesReleases(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/release/", func(w http.ResponseWriter, r *http.Request) {
		file := strings.TrimPrefix(r.URL.Path, "/release/")
		switch file {
		case "stable.txt", "stable-1.4.txt":
			fmt.Fprint(w, "v1.4.5")
		case "stable-1.2.txt":
			fmt.Fprint(w, "v1.2.7")
		case "stable-1.3.txt":
			fmt.Fprint(w, "v1.3.2")
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := srv.Client()

	releases, err := fetchKubernetesReleasesAt(client, srv.URL+"/release")
	if err != nil {
		t.Fatalf("fetchKubernetesReleases: %v", err)
	}

	want := []string{"v1.4.5", "v1.3.2", "v1.2.7"}
	if len(releases) != len(want) {
		t.Fatalf("got %d releases, want %d: %+v", len(releases), len(want), releases)
	}
	for i, v := range want {
		if releases[i].Version != v {
			t.Errorf("releases[%d] = %q, want %q", i, releases[i].Version, v)
		}
	}
}

func TestCompareKubernetesVersions(t *testing.T) {
	cases := [][2]string{
		{"v1.37.0", "v1.9.3"},
		{"v1.30.1", "v1.30.0"},
		{"v1.2.7", "v1.2.7"},
	}
	for _, c := range cases {
		if got := compareKubernetesVersions(c[0], c[1]); got < 0 {
			t.Errorf("compare(%s, %s) = %d, want > 0", c[0], c[1], got)
		}
	}
}

func TestKubernetesBinaryTarget(t *testing.T) {
	for _, tc := range []struct{ os, arch, binary string }{
		{"linux", "amd64", "kubectl"},
		{"linux", "arm64", "kubectl"},
		{"darwin", "arm64", "kubectl"},
		{"windows", "amd64", "kubectl.exe"},
	} {
		osToken, archToken, binary, ok := kubernetesBinaryTarget(tc.os, tc.arch)
		if !ok || osToken != tc.os || archToken != tc.arch || binary != tc.binary {
			t.Errorf("%s/%s: got %q %q %q %v", tc.os, tc.arch, osToken, archToken, binary, ok)
		}
	}
	if _, _, _, ok := kubernetesBinaryTarget("windows", "arm64"); !ok {
		t.Error("windows/arm64 should be supported")
	}
	if _, _, _, ok := kubernetesBinaryTarget("plan9", "amd64"); ok {
		t.Error("plan9 should not be supported")
	}
}

func TestKubernetesArtifact(t *testing.T) {
	file, ok := kubernetesArtifact("v1.37.0", "windows", "amd64", "https://dl.k8s.io/release/v1.37.0/bin/windows/amd64/kubectl.exe", "abcd")
	if !ok {
		t.Fatal("expected artifact")
	}
	if file.Filename != "kubectl.exe" || file.OS != "windows" || file.Arch != "amd64" || file.Version != "v1.37.0" || file.Kind != "archive" {
		t.Errorf("unexpected file: %+v", file)
	}
	if file.URL != "https://dl.k8s.io/release/v1.37.0/bin/windows/amd64/kubectl.exe" {
		t.Errorf("unexpected URL: %s", file.URL)
	}
}

func TestFetchKubernetesSHA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "4721b614a67bb4932a0369e61f4a323d8c6ca00943d3a2ff14837c124da06f0e  kubectl.exe")
	}))
	defer srv.Close()

	got, err := fetchKubernetesSHA(srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("fetchKubernetesSHA: %v", err)
	}
	if want := "4721b614a67bb4932a0369e61f4a323d8c6ca00943d3a2ff14837c124da06f0e"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
