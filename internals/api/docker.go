package api

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"sort"
)

const dockerDownloadBase = "https://download.docker.com"

var dockerArchiveRe = regexp.MustCompile(`^docker-([0-9]+\.[0-9]+\.[0-9]+)\.(tgz|zip)$`)
var dockerHrefRe = regexp.MustCompile(`(?i)\bhref\s*=\s*["']([^"']+)["']`)

type DockerBundle struct {
	Label      string
	Components []string
	Notice     string
}

func DockerBundleForMachine(goos, goarch string) (DockerBundle, error) {
	if _, _, err := dockerTarget(goos, goarch); err != nil {
		return DockerBundle{}, err
	}
	common := "Buildx and Compose are not included. Services are not configured or started."
	switch goos {
	case "linux":
		return DockerBundle{
			Label:      "Docker Engine static bundle",
			Components: []string{"docker", "dockerd", "containerd", "runc"},
			Notice:     common + " Configure the daemon separately; distribution packages are recommended for production.",
		}, nil
	case "windows":
		return DockerBundle{
			Label:      "Docker Engine (native Windows containers)",
			Components: []string{"docker.exe", "dockerd.exe"},
			Notice:     common + " Not Docker Desktop or a Linux-container engine. Docker Desktop is recommended for Windows 10/11.",
		}, nil
	default:
		return DockerBundle{
			Label:      "Docker CLI only",
			Components: []string{"docker"},
			Notice:     common + " No local Engine is included. Use a remote Engine or Docker Desktop on macOS.",
		}, nil
	}
}

func dockerTarget(goos, goarch string) (string, string, error) {
	platform, arch, ext := "", "", "tgz"
	switch goos {
	case "linux":
		platform = "linux"
		arch = map[string]string{"amd64": "x86_64", "arm64": "aarch64", "ppc64le": "ppc64le", "s390x": "s390x"}[goarch]
	case "darwin":
		platform = "mac"
		arch = map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[goarch]
	case "windows":
		platform, ext = "win", "zip"
		if goarch == "amd64" {
			arch = "x86_64"
		}
	}
	if platform == "" || arch == "" {
		return "", "", fmt.Errorf("docker has no official static binaries for %s/%s", goos, goarch)
	}
	return fmt.Sprintf("%s/%s/static/stable/%s/", dockerDownloadBase, platform, arch), ext, nil
}

func FetchDockerReleases() ([]Release, error) {
	return fetchDockerReleases(&http.Client{Timeout: requestHTTPTimeout}, runtime.GOOS, runtime.GOARCH)
}

func fetchDockerReleases(client *http.Client, goos, goarch string) ([]Release, error) {
	base, ext, err := dockerTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	resp, err := client.Get(base)
	if err != nil {
		return nil, fmt.Errorf("docker version list unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker version list unavailable (server returned %s)", resp.Status)
	}
	const maxListingSize = 8 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxListingSize+1))
	if err != nil {
		return nil, fmt.Errorf("docker version list unavailable: %w", err)
	}
	if len(body) > maxListingSize {
		return nil, fmt.Errorf("docker version list is unexpectedly large")
	}
	releases := parseDockerListing(string(body), base, ext, goos, goarch)
	if len(releases) == 0 {
		return nil, fmt.Errorf("no docker releases found for %s/%s", goos, goarch)
	}
	return releases, nil
}

func parseDockerListing(body, base, ext, goos, goarch string) []Release {
	seen := make(map[string]bool)
	var releases []Release
	for _, link := range dockerHrefRe.FindAllStringSubmatch(body, -1) {
		match := dockerArchiveRe.FindStringSubmatch(link[1])
		if match == nil || match[2] != ext || seen[match[1]] {
			continue
		}
		version := match[1]
		seen[version] = true
		releases = append(releases, Release{Version: version, Stable: true, Files: []File{{
			Filename: link[1], OS: goos, Arch: goarch, Version: version,
			Kind: "archive", URL: base + link[1],
		}}})
	}
	sort.Slice(releases, func(i, j int) bool {
		return compareVersionsNonEmpty(releases[i].Version, releases[j].Version) > 0
	})
	return releases
}

func FetchDockerArtifact(version string) (File, error) {
	if !dockerArchiveRe.MatchString("docker-" + version + ".tgz") {
		return File{}, fmt.Errorf("invalid docker version %q", version)
	}
	releases, err := FetchDockerReleases()
	if err != nil {
		return File{}, err
	}
	for _, release := range releases {
		if release.Version == version {
			return release.Files[0], nil
		}
	}
	return File{}, fmt.Errorf("no docker %s download available for %s/%s", version, runtime.GOOS, runtime.GOARCH)
}
