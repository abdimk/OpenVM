package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
)

const (
	// llvmReleasesURL is the GitHub releases API for the LLVM project, which
	// publishes the official prebuilt toolchain packages as release assets.
	llvmReleasesURL = "https://api.github.com/repos/llvm/llvm-project/releases"
	// llvmMaxPages caps how many 100-entry pages we walk. LLVM has well over
	// 100 tagged releases, so a bounded paginated walk is required.
	llvmMaxPages = 5
)

// FetchLLVMReleases returns every published, non-release-candidate LLVM
// release that ships a downloadable toolchain package matching the current
// machine's OS and architecture, newest first.
func FetchLLVMReleases() ([]Release, error) {
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

	return llvmReleasesToModel(all), nil
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

// llvmReleasesToModel converts raw GitHub release data into OpenVM's
// api.Release model. Drafts, prereleases (release candidates) and versions
// with no package for the current machine are dropped.
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

// llvmAssetForMachine picks the best archive asset from a release for the
// current OS/architecture. Among the matching assembles the most preferred
// compression (zstd over xz over gzip over zip) is chosen.
func llvmAssetForMachine(version string, assets []GitHubAsset) (File, bool) {
	return llvmAssetFor(version, assets, runtime.GOOS, runtime.GOARCH)
}

// llvmAssetFor is llvmAssetForMachine with an explicit target, so the
// matcher can be exercised for every platform in tests.
func llvmAssetFor(version string, assets []GitHubAsset, targetOS, targetArch string) (File, bool) {
	bestRank := int(^uint(0) >> 1)
	var best *GitHubAsset

	for i := range assets {
		a := &assets[i]

		rank := llvmArchiveRank(a.Name)
		if rank == 0 {
			continue // not an installable archive
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

// llvmArchiveRank maps a downloadable asset's extension to a preference
// rank; a rank of 0 means "not an installable package".
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

// llvmMatches reports whether an LLVM release asset is an official prebuilt
// toolchain package for the given OS/architecture, ignoring installers
// (.msi/.exe), source tarballs, docs and companion metadata files
// (.jsonl, .sig).
func llvmMatches(name, targetOS, targetArch string) bool {
	lower := strings.ToLower(name)

	// Only amalgamated toolchain packages: "LLVM-*" and "clang+llvm-*",
	// and only installable archives (rank 0 covers .msi/.exe/.jsonl/.sig).
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