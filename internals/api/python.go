package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	pepsReleasesURL = "https://peps.python.org/api/python-releases.json"

	pythonFTPBase = "https://www.python.org/ftp/python"

	pythonMinMinor = 11

	pythonStandaloneRepo    = "astral-sh/python-build-standalone"
	pythonStandaloneBase    = "https://github.com/" + pythonStandaloneRepo + "/releases/download"
	pythonStandaloneTagsURL = "https://api.github.com/repos/" + pythonStandaloneRepo + "/git/matching-refs/tags/20"
)

var pythonFinalVersionRe = regexp.MustCompile(`^\d+\.\d+\.\d+( final)?$`)

var pythonFTPDirRe = regexp.MustCompile(`^(\d+\.\d+\.\d+)/?$`)

type pepsStage struct {
	Stage string `json:"stage"`
	State string `json:"state"`
	Date  string `json:"date"`
	Note  string `json:"note"`
}

type pythonWindowsManifest struct {
	Versions []pythonWindowsPackage `json:"versions"`
}

type pythonWindowsPackage struct {
	ID   string            `json:"id"`
	URL  string            `json:"url"`
	Hash pythonPackageHash `json:"hash"`
}

type pythonPackageHash struct {
	SHA256 string `json:"sha256"`
}

type PythonReleaseInfo struct {
	Version string
	Date    string
}

func FetchPythonVersions() ([]PythonReleaseInfo, error) {
	return fetchPythonVersions()
}

func fetchPythonVersions() ([]PythonReleaseInfo, error) {
	infos, pepsErr := fetchPythonVersionsFromPeps()
	if pepsErr == nil && len(infos) > 0 {
		return infos, nil
	}

	versions, ftpErr := fetchPythonVersionsFromFTP()
	if ftpErr != nil {
		return nil, fmt.Errorf("fetching Python versions: peps: %v; ftp: %v", pepsErr, ftpErr)
	}

	infos = make([]PythonReleaseInfo, 0, len(versions))
	for _, v := range versions {
		infos = append(infos, PythonReleaseInfo{Version: v})
	}
	return infos, nil
}

func fetchPythonVersionsFromPeps() ([]PythonReleaseInfo, error) {
	client := &http.Client{Timeout: requestHTTPTimeout}
	resp, err := client.Get(pepsReleasesURL)
	if err != nil {
		return nil, fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	seriesRaw, ok := doc["releases"]
	if !ok {
		return nil, fmt.Errorf("missing \"releases\" section")
	}
	var series map[string]json.RawMessage
	if err := json.Unmarshal(seriesRaw, &series); err != nil {
		return nil, fmt.Errorf("parsing releases: %w", err)
	}

	return pythonReleaseInfosFromSeries(series), nil
}

func fetchPythonVersionsFromFTP() ([]string, error) {
	url := pythonFTPBase + "/"
	client := &http.Client{Timeout: requestHTTPTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching FTP index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading FTP index: %w", err)
	}

	seen := make(map[string]bool)
	for _, m := range pythonHrefRe.FindAllStringSubmatch(string(body), -1) {
		if sub := pythonFTPDirRe.FindStringSubmatch(m[1]); sub != nil {
			seen[sub[1]] = true
		}
	}

	versions := make([]string, 0, len(seen))
	for v := range seen {
		if pythonSeriesSupported(v) {
			versions = append(versions, v)
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareVersionsNonEmpty(versions[i], versions[j]) > 0
	})
	return versions, nil
}

func pythonReleaseInfosFromSeries(series map[string]json.RawMessage) []PythonReleaseInfo {
	seen := make(map[string]pepsStage)
	for _, raw := range series {
		var stages []pepsStage
		if err := json.Unmarshal(raw, &stages); err != nil {
			continue
		}
		for _, s := range stages {
			if s.State != "actual" || !pythonFinalVersionRe.MatchString(s.Stage) {
				continue
			}
			version := strings.TrimSuffix(s.Stage, " final")
			if prev, ok := seen[version]; !ok || s.Date > prev.Date {
				seen[version] = s
			}
		}
	}

	infos := make([]PythonReleaseInfo, 0, len(seen))
	for v, s := range seen {
		if !pythonSeriesSupported(v) {
			continue
		}
		infos = append(infos, PythonReleaseInfo{Version: v, Date: s.Date})
	}

	sort.Slice(infos, func(i, j int) bool {
		return compareVersionsNonEmpty(infos[i].Version, infos[j].Version) > 0
	})

	return infos
}

func pythonSeriesSupported(version string) bool {
	majorStr, rest, ok := strings.Cut(version, ".")
	if !ok {
		return false
	}
	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return false
	}
	minorStr, _, _ := strings.Cut(rest, ".")
	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		return false
	}
	return major > 3 || (major == 3 && minor >= pythonMinMinor)
}

func compareVersionsNonEmpty(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	n := max(len(as), len(bs))
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func FetchPythonArtifact(version string) (File, error) {
	file, ok := pythonArtifactForMachine(version, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return File{}, fmt.Errorf("no downloadable Python %s artifact for %s/%s", version, runtime.GOOS, runtime.GOARCH)
	}
	return file, nil
}

func pythonArtifactForMachine(version, targetOS, targetArch string) (File, bool) {
	switch targetOS {
	case "windows":
		return pythonWindowsArtifact(version, targetArch)
	default:
		return pythonStandaloneConfig{
			client:  &http.Client{Timeout: requestHTTPTimeout},
			tagsURL: pythonStandaloneTagsURL,
			base:    pythonStandaloneBase,
		}.artifact(version, targetOS, targetArch)
	}
}

func pythonWindowsArtifact(version, targetArch string) (File, bool) {
	manifest, err := fetchPythonWindowsManifest(version)
	if err != nil {
		return File{}, false
	}
	return matchWindowsManifest(manifest, version, targetArch)
}

func matchWindowsManifest(manifest *pythonWindowsManifest, version, targetArch string) (File, bool) {
	archToken := pythonArchToken(targetArch)
	if archToken == "" {
		return File{}, false
	}
	filename := fmt.Sprintf("python-%s-%s.zip", version, archToken)

	for _, pkg := range manifest.Versions {
		if !strings.HasPrefix(pkg.ID, "pythoncore-") {
			continue
		}
		if !strings.HasSuffix(pkg.URL, filename) {
			continue
		}
		return File{
			Filename: filename,
			OS:       "windows",
			Arch:     targetArch,
			Version:  version,
			SHA256:   pkg.Hash.SHA256,
			Kind:     "archive",
			URL:      pkg.URL,
		}, true
	}
	return File{}, false
}

func pythonArchToken(arch string) string {
	switch arch {
	case "amd64", "arm64":
		return arch
	default:
		return ""
	}
}

func fetchPythonWindowsManifest(version string) (*pythonWindowsManifest, error) {
	url := fmt.Sprintf("%s/%s/windows-%s.json", pythonFTPBase, version, version)
	client := &http.Client{Timeout: requestHTTPTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var manifest pythonWindowsManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

var pythonHrefRe = regexp.MustCompile(`href="([^"]+)"`)

var pythonStandaloneTagRe = regexp.MustCompile(`refs/tags/(20[0-9]{6})$`)

var (
	pythonStandaloneTagsMu    sync.Mutex
	pythonStandaloneTagsCache []string
	pythonStandaloneTagsAt    time.Time
)

type pythonStandaloneConfig struct {
	client  *http.Client
	tagsURL string
	base    string
}

func pythonStandaloneTriple(goos, goarch string) (string, bool) {
	switch goos {
	case "linux":
		switch goarch {
		case "amd64":
			return "x86_64-unknown-linux-gnu", true
		case "arm64":
			return "aarch64-unknown-linux-gnu", true
		case "arm":
			return "armv7-unknown-linux-gnueabihf", true
		case "ppc64le":
			return "ppc64le-unknown-linux-gnu", true
		case "s390x":
			return "s390x-unknown-linux-gnu", true
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "x86_64-apple-darwin", true
		case "arm64":
			return "aarch64-apple-darwin", true
		}
	}
	return "", false
}

func (c pythonStandaloneConfig) artifact(version, targetOS, targetArch string) (File, bool) {
	triple, ok := pythonStandaloneTriple(targetOS, targetArch)
	if !ok {
		return File{}, false
	}

	tags, err := c.tags()
	if err != nil || len(tags) == 0 {
		return File{}, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var mu sync.Mutex
	best := ""
	var wg sync.WaitGroup
	sem := make(chan struct{}, 12)

	for _, tag := range tags {
		wg.Add(1)
		go func(tag string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			if !c.hasAsset(pythonStandaloneAssetURL(c.base, tag, version, triple)) {
				return
			}
			mu.Lock()
			if best == "" || tag > best {
				best = tag
			}
			mu.Unlock()
		}(tag)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-ctx.Done():
		return File{}, false
	case <-done:
	}

	if best == "" {
		return File{}, false
	}

	name := pythonStandaloneAssetName(version, best, triple)

	sha := ""
	if checksums, err := c.shaSums(best); err == nil {
		sha = checksums[name]
	}

	return File{
		Filename: name,
		OS:       targetOS,
		Arch:     targetArch,
		Version:  version,
		SHA256:   sha,
		Kind:     "archive",
		URL:      pythonStandaloneAssetURL(c.base, best, version, triple),
	}, true
}

func (c pythonStandaloneConfig) tags() ([]string, error) {
	if c.tagsURL == pythonStandaloneTagsURL {
		pythonStandaloneTagsMu.Lock()
		defer pythonStandaloneTagsMu.Unlock()
		if len(pythonStandaloneTagsCache) > 0 && time.Since(pythonStandaloneTagsAt) < 10*time.Minute {
			return pythonStandaloneTagsCache, nil
		}
	}

	client := c.client
	if client == nil {
		client = &http.Client{Timeout: requestHTTPTimeout}
	}

	resp, err := client.Get(c.tagsURL)
	if err != nil {
		return nil, fmt.Errorf("fetching python build-standalone tags: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching python build-standalone tags: unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var refs []struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(body, &refs); err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var tags []string
	for _, r := range refs {
		if m := pythonStandaloneTagRe.FindStringSubmatch(r.Ref); m != nil && !seen[m[1]] {
			seen[m[1]] = true
			tags = append(tags, m[1])
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(tags)))

	if c.tagsURL == pythonStandaloneTagsURL {
		pythonStandaloneTagsCache = tags
		pythonStandaloneTagsAt = time.Now()
	}
	return tags, nil
}

func (c pythonStandaloneConfig) hasAsset(url string) bool {
	client := c.client
	if client == nil {
		client = &http.Client{Timeout: requestHTTPTimeout}
	}
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c pythonStandaloneConfig) shaSums(tag string) (map[string]string, error) {
	client := c.client
	if client == nil {
		client = &http.Client{Timeout: requestHTTPTimeout}
	}

	resp, err := client.Get(fmt.Sprintf("%s/%s/SHA256SUMS", c.base, tag))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching python build-standalone checksums: unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	checksums := make(map[string]string)
	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		filename := strings.TrimPrefix(parts[1], "./")
		checksums[filename] = parts[0]
	}
	return checksums, nil
}

func pythonStandaloneAssetName(version, tag, triple string) string {
	return fmt.Sprintf("cpython-%s+%s-%s-install_only.tar.gz", version, tag, triple)
}

func pythonStandaloneAssetURL(base, tag, version, triple string) string {
	return fmt.Sprintf("%s/%s/%s", base, tag, pythonStandaloneAssetName(version, tag, triple))
}
