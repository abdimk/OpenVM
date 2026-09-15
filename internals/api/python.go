package api

import (
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
)

const (
	pepsReleasesURL = "https://peps.python.org/api/python-releases.json"

	pythonFTPBase = "https://www.python.org/ftp/python"

	pythonMinMinor = 11

	pythonManifestWorkers = 6
)

var pythonFinalVersionRe = regexp.MustCompile(`^\d+\.\d+\.\d+( final)?$`)

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

func FetchPythonReleases() ([]Release, error) {
	versions, err := fetchPythonVersions()
	if err != nil {
		return nil, err
	}

	slots := make([]*Release, len(versions))
	sem := make(chan struct{}, pythonManifestWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, version := range versions {
		version := version
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			file, ok := pythonArtifactForMachine(version, runtime.GOOS, runtime.GOARCH)
			if !ok {
				return
			}
			mu.Lock()
			slots[slot] = &Release{
				Version: version,
				Stable:  true,
				Files:   []File{file},
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	out := make([]Release, 0, len(versions))
	for _, r := range slots {
		if r != nil {
			out = append(out, *r)
		}
	}
	return out, nil
}

func fetchPythonVersions() ([]string, error) {
	url := pepsReleasesURL

	client := &http.Client{Timeout: requestHTTPTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching Python releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching Python releases: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading Python releases response: %w", err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parsing Python releases response: %w", err)
	}

	seriesRaw, ok := doc["releases"]
	if !ok {
		return nil, fmt.Errorf("parsing Python releases response: missing \"releases\" section")
	}
	var series map[string]json.RawMessage
	if err := json.Unmarshal(seriesRaw, &series); err != nil {
		return nil, fmt.Errorf("parsing Python releases response: %w", err)
	}

	return pythonVersionsFromSeries(series), nil
}

func pythonVersionsFromSeries(series map[string]json.RawMessage) []string {
	seen := make(map[string]bool)
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
			seen[version] = true
		}
	}

	versions := make([]string, 0, len(seen))
	for v := range seen {
		if !pythonSeriesSupported(v) {
			continue
		}
		versions = append(versions, v)
	}

	sort.Slice(versions, func(i, j int) bool {
		return compareVersionsNonEmpty(versions[i], versions[j]) > 0
	})

	return versions
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

func pythonArtifactForMachine(version, targetOS, targetArch string) (File, bool) {
	switch targetOS {
	case "windows":
		return pythonWindowsArtifact(version, targetArch)
	default:

		return File{}, false
	}
}

func pythonWindowsArtifact(version, targetArch string) (File, bool) {
	if manifest, err := fetchPythonWindowsManifest(version); err == nil {
		if file, ok := matchWindowsManifest(manifest, version, targetArch); ok {
			return file, true
		}
	}

	files, err := fetchPythonDirIndex(version)
	if err != nil {
		return File{}, false
	}
	archToken := pythonArchToken(targetArch)
	if archToken == "" {
		return File{}, false
	}
	filename := fmt.Sprintf("python-%s-%s.zip", version, archToken)
	for _, name := range files {
		if name == filename {
			return File{
				Filename: filename,
				OS:       "windows",
				Arch:     targetArch,
				Version:  version,
				Kind:     "archive",
				URL:      fmt.Sprintf("%s/%s/%s", pythonFTPBase, version, name),
			}, true
		}
	}
	return File{}, false
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

func fetchPythonDirIndex(version string) ([]string, error) {
	url := fmt.Sprintf("%s/%s/", pythonFTPBase, version)
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

	var files []string
	for _, m := range pythonHrefRe.FindAllStringSubmatch(string(body), -1) {
		name := m[1]
		if name == "../" || strings.HasPrefix(name, "/") || strings.Contains(name, "?") {
			continue
		}
		files = append(files, name)
	}
	return files, nil
}
