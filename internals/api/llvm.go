package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	llvmReleasesURL = "https://api.github.com/repos/llvm/llvm-project/releases"

	llvmMaxPages = 5

	llvmCacheTTL = 10 * time.Minute
)

var (
	llvmCacheMu   sync.Mutex
	llvmCacheTime time.Time
	llvmCached    []Release
)

func FetchLLVMReleases() ([]Release, error) {
	llvmCacheMu.Lock()
	defer llvmCacheMu.Unlock()

	if time.Since(llvmCacheTime) < llvmCacheTTL {
		return llvmCached, nil
	}

	var all []GitHubRelease
	for page := 1; page <= llvmMaxPages; page++ {
		url := fmt.Sprintf("%s?per_page=100&page=%d", llvmReleasesURL, page)
		batch, err := fetchGitHubReleases(url)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}

	llvmCached = llvmReleasesToModel(all)
	llvmCacheTime = time.Now()
	return llvmCached, nil
}

func fetchGitHubReleases(url string) ([]GitHubRelease, error) {
	client := &http.Client{Timeout: requestHTTPTimeout}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating LLVM releases request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "OpenVM-version-manager")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching LLVM releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching LLVM releases: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading LLVM releases response: %w", err)
	}

	var releases []GitHubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("parsing LLVM releases response: %w", err)
	}

	return releases, nil
}

func llvmReleasesToModel(releases []GitHubRelease) []Release {
	out := make([]Release, 0, len(releases))

	for _, gr := range releases {
		version := strings.TrimPrefix(gr.TagName, "llvmorg-")
		if version == "" || strings.Contains(version, "-rc") || strings.Contains(version, ".final") {
			continue
		}
		if gr.Draft || gr.Prerelease {
			continue
		}

		file, ok := llvmAssetForMachine(version, gr.Assets)
		if !ok {
			continue
		}

		out = append(out, Release{
			Version: version,
			Stable:  true,
			Files:   []File{file},
		})
	}

	return out
}

func llvmAssetForMachine(version string, assets []GitHubAsset) (File, bool) {
	return llvmAssetFor(version, assets, runtime.GOOS, runtime.GOARCH)
}

func llvmAssetFor(version string, assets []GitHubAsset, targetOS, targetArch string) (File, bool) {
	bestRank := int(^uint(0) >> 1)
	var best *GitHubAsset

	for i := range assets {
		a := &assets[i]

		rank := llvmArchiveRank(a.Name)
		if rank == 0 {
			continue
		}
		if !llvmMatches(a.Name, targetOS, targetArch) {
			continue
		}
		if rank < bestRank {
			bestRank = rank
			best = a
		}
	}

	if best == nil {
		return File{}, false
	}

	return File{
		Filename: best.Name,
		OS:       targetOS,
		Arch:     targetArch,
		Version:  version,
		Size:     best.Size,
		Kind:     "archive",
		URL:      best.URL,
	}, true
}

func llvmArchiveRank(name string) int {
	switch {
	case strings.HasSuffix(name, ".tar.zst"):
		return 1
	case strings.HasSuffix(name, ".tar.xz"):
		return 2
	case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"):
		return 3
	case strings.HasSuffix(name, ".zip"):
		return 4
	default:
		return 0
	}
}

func llvmMatches(name, targetOS, targetArch string) bool {
	lower := strings.ToLower(name)

	if (!strings.HasPrefix(lower, "llvm-") && !strings.HasPrefix(lower, "clang+llvm-")) ||
		llvmArchiveRank(lower) == 0 {
		return false
	}
	if strings.Contains(lower, ".src.tar") || strings.Contains(lower, "_doxygen") {
		return false
	}

	var assetOS string
	switch {
	case strings.Contains(lower, "windows"), strings.Contains(lower, "msvc"):
		assetOS = "windows"
	case strings.Contains(lower, "linux"):
		assetOS = "linux"
	case strings.Contains(lower, "macos"), strings.Contains(lower, "darwin"):
		assetOS = "darwin"
	default:
		return false
	}

	var assetArch string
	switch {
	case strings.Contains(lower, "aarch64"), strings.Contains(lower, "arm64"):
		assetArch = "arm64"
	case strings.Contains(lower, "x86_64"), strings.Contains(lower, "x64"),
		strings.Contains(lower, "amd64"), strings.Contains(lower, "win64"):
		assetArch = "amd64"
	default:
		return false
	}

	return assetOS == targetOS && assetArch == targetArch
}
