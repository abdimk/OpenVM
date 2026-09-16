package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"strings"
)

const (
	nodeDistBase = "https://nodejs.org/dist"

	nodeDistIndexURL = nodeDistBase + "/index.json"

	nodeMinMajor = 18
)

var nodeFinalVersionRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

type nodeRelease struct {
	Version string `json:"version"`
	Date    string `json:"date"`
	LTS     any    `json:"lts"`
}

type NodeReleaseInfo struct {
	Version string
	Date    string
	LTS     string
}

func FetchNodeVersions() ([]NodeReleaseInfo, error) {
	client := &http.Client{Timeout: requestHTTPTimeout}
	resp, err := client.Get(nodeDistIndexURL)
	if err != nil {
		return nil, fmt.Errorf("fetching Node versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching Node versions: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading Node versions response: %w", err)
	}

	var releases []nodeRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("parsing Node versions response: %w", err)
	}

	return nodeReleaseInfosFromReleases(releases), nil
}

func nodeReleaseInfosFromReleases(releases []nodeRelease) []NodeReleaseInfo {
	var infos []NodeReleaseInfo
	for _, r := range releases {
		if !nodeFinalVersionRe.MatchString(r.Version) {
			continue
		}
		if !nodeMajorSupported(r.Version) {
			continue
		}
		info := NodeReleaseInfo{
			Version: nodeBare(r.Version),
			Date:    r.Date,
		}
		if s, ok := r.LTS.(string); ok && s != "" {
			info.LTS = s
		}
		infos = append(infos, info)
	}

	sortNodeVersionsDesc(infos)
	return infos
}

func nodeMajorSupported(version string) bool {
	m := nodeFinalVersionRe.FindStringSubmatch(version)
	if m == nil {
		return false
	}
	major := 0
	for _, c := range m[1] {
		major = major*10 + int(c-'0')
	}
	return major >= nodeMinMajor
}

func sortNodeVersionsDesc(infos []NodeReleaseInfo) {
	for i := 1; i < len(infos); i++ {
		for j := i; j > 0 && compareVersionsNonEmpty(infos[j].Version, infos[j-1].Version) > 0; j-- {
			infos[j], infos[j-1] = infos[j-1], infos[j]
		}
	}
}

func nodeBare(version string) string {
	return strings.TrimPrefix(version, "v")
}

func nodeTag(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func FetchNodeArtifact(version string) (File, error) {
	file, ok := nodeArtifactForMachine(version, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return File{}, fmt.Errorf("no downloadable Node %s artifact for %s/%s", version, runtime.GOOS, runtime.GOARCH)
	}
	return file, nil
}

func nodeArtifactForMachine(version, targetOS, targetArch string) (File, bool) {
	token, exts, ok := nodeArtifactTarget(targetOS, targetArch)
	if !ok {
		return File{}, false
	}

	tag := nodeTag(version)

	checksums, err := fetchNodeSHASUMS(tag)
	if err != nil {
		return File{}, false
	}

	for _, ext := range exts {
		filename := fmt.Sprintf("node-%s-%s%s", tag, token, ext)
		sha, ok := checksums[filename]
		if !ok {
			continue
		}
		return File{
			Filename: filename,
			OS:       targetOS,
			Arch:     targetArch,
			Version:  version,
			SHA256:   sha,
			Kind:     "archive",
			URL:      fmt.Sprintf("%s/%s/%s", nodeDistBase, tag, filename),
		}, true
	}
	return File{}, false
}

func nodeArtifactTarget(goos, goarch string) (token string, exts []string, ok bool) {
	switch goos {
	case "windows":
		switch goarch {
		case "amd64":
			return "win-x64", []string{".zip"}, true
		case "arm64":
			return "win-arm64", []string{".zip"}, true
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "darwin-x64", []string{".tar.gz"}, true
		case "arm64":
			return "darwin-arm64", []string{".tar.gz"}, true
		}
	case "linux":
		switch goarch {
		case "amd64":
			return "linux-x64", []string{".tar.xz", ".tar.gz"}, true
		case "arm64":
			return "linux-arm64", []string{".tar.xz", ".tar.gz"}, true
		}
	}
	return "", nil, false
}

func fetchNodeSHASUMS(tag string) (map[string]string, error) {
	url := fmt.Sprintf("%s/%s/SHASUMS256.txt", nodeDistBase, tag)
	client := &http.Client{Timeout: requestHTTPTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching SHASUMS256.txt: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseNodeSHASUMS(string(body)), nil
}

func parseNodeSHASUMS(text string) map[string]string {
	checksums := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		filename := strings.TrimPrefix(parts[1], "*")
		checksums[filename] = parts[0]
	}
	return checksums
}
