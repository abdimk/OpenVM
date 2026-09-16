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
)

const (
	pepsReleasesURL = "https://peps.python.org/api/python-releases.json"

	pythonFTPBase = "https://www.python.org/ftp/python"

	pythonMinMinor = 11
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
		return File{}, false
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
