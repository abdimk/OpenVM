package api

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	kubernetesReleaseBase = "https://dl.k8s.io/release"

	kubernetesStableURL = kubernetesReleaseBase + "/stable.txt"
)

var kubernetesVersionRe = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`)

var kubernetesSHARe = regexp.MustCompile(`^[0-9a-f]{64}$`)

func FetchKubernetesReleases() ([]Release, error) {
	return fetchKubernetesReleasesAt(&http.Client{Timeout: requestHTTPTimeout}, kubernetesReleaseBase)
}

func fetchKubernetesReleasesAt(client *http.Client, base string) ([]Release, error) {
	latestRaw, err := fetchKubernetesStable(client, base+"/stable.txt")
	if err != nil {
		return nil, err
	}
	latest := strings.TrimSpace(latestRaw)
	if !kubernetesVersionRe.MatchString(latest) {
		return nil, fmt.Errorf("unexpected Kubernetes stable version %q", latest)
	}

	major, latestMinor, _, _ := kubernetesMinorParts(latest)

	var mu sync.Mutex
	seen := map[string]bool{}
	var releases []Release

	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for minor := 1; minor <= latestMinor; minor++ {
		wg.Add(1)
		go func(minor int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			url := fmt.Sprintf("%s/stable-%d.%d.txt", base, major, minor)
			version, err := fetchKubernetesStable(client, url)
			if err != nil || !kubernetesVersionRe.MatchString(version) {
				return
			}

			mu.Lock()
			defer mu.Unlock()
			if seen[version] {
				return
			}
			seen[version] = true
			releases = append(releases, Release{Version: version, Stable: true})
		}(minor)
	}
	wg.Wait()

	if !seen[latest] {
		releases = append(releases, Release{Version: latest, Stable: true})
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no Kubernetes releases found")
	}

	sort.Slice(releases, func(i, j int) bool {
		return compareKubernetesVersions(releases[i].Version, releases[j].Version) > 0
	})
	return releases, nil
}

func fetchKubernetesStable(client *http.Client, urls ...string) (string, error) {
	if len(urls) == 0 {
		urls = []string{kubernetesStableURL}
	}
	resp, err := client.Get(urls[0])
	if err != nil {
		return "", fmt.Errorf("fetching Kubernetes version: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching Kubernetes version: unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	if err != nil {
		return "", fmt.Errorf("reading Kubernetes version: %w", err)
	}
	return strings.TrimSpace(string(body)), nil
}

func kubernetesMinorParts(version string) (major, minor, patch int, ok bool) {
	m := kubernetesVersionRe.FindStringSubmatch(version)
	if m == nil {
		return 0, 0, 0, false
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	patch, _ = strconv.Atoi(m[3])
	return major, minor, patch, true
}

func compareKubernetesVersions(a, b string) int {
	amaj, amin, apat, aok := kubernetesMinorParts(a)
	bmaj, bmin, bpat, bok := kubernetesMinorParts(b)
	if !aok || !bok {
		return strings.Compare(a, b)
	}
	for _, pair := range [][2]int{{amaj, bmaj}, {amin, bmin}, {apat, bpat}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return 0
}

func FetchKubernetesArtifact(version string) (File, error) {
	if !kubernetesVersionRe.MatchString(version) {
		return File{}, fmt.Errorf("invalid Kubernetes version %q", version)
	}

	osToken, archToken, binary, ok := kubernetesBinaryTarget(runtime.GOOS, runtime.GOARCH)
	if !ok {
		return File{}, fmt.Errorf("kubernetes has no kubectl binary for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	url := fmt.Sprintf("%s/%s/bin/%s/%s/%s", kubernetesReleaseBase, version, osToken, archToken, binary)

	sha, err := fetchKubernetesSHA(&http.Client{Timeout: requestHTTPTimeout}, url+".sha256")
	if err != nil {
		return File{}, err
	}

	file, ok := kubernetesArtifact(version, runtime.GOOS, runtime.GOARCH, url, sha)
	if !ok {
		return File{}, fmt.Errorf("kubernetes has no kubectl binary for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return file, nil
}

func fetchKubernetesSHA(client *http.Client, url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetching Kubernetes checksum: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching Kubernetes checksum: unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	if err != nil {
		return "", fmt.Errorf("reading Kubernetes checksum: %w", err)
	}
	for _, field := range strings.Fields(string(body)) {
		if kubernetesSHARe.MatchString(field) {
			return field, nil
		}
	}
	return "", fmt.Errorf("unexpected Kubernetes checksum file: %q", strings.TrimSpace(string(body)))
}

func kubernetesArtifact(version, goos, goarch, url, sha string) (File, bool) {
	_, _, binary, ok := kubernetesBinaryTarget(goos, goarch)
	if !ok {
		return File{}, false
	}
	return File{
		Filename: binary,
		OS:       goos,
		Arch:     goarch,
		Version:  version,
		SHA256:   sha,
		Kind:     "archive",
		URL:      url,
	}, true
}

func kubernetesBinaryTarget(goos, goarch string) (osToken, archToken, binary string, ok bool) {
	binary = "kubectl"
	switch goos {
	case "linux":
		switch goarch {
		case "amd64", "arm64", "arm", "386", "ppc64le", "s390x":
			return goos, goarch, binary, true
		}
	case "darwin":
		switch goarch {
		case "amd64", "arm64":
			return goos, goarch, binary, true
		}
	case "windows":
		binary = "kubectl.exe"
		switch goarch {
		case "amd64", "arm64", "386":
			return goos, goarch, binary, true
		}
	}
	return "", "", "", false
}
