package api

import "runtime"

type File struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Kind     string `json:"kind"`
	// URL is the direct download link for the file. Go files leave it empty
	// (the URL is derived from go.dev), while LLVM assets carry GitHub's
	// browser_download_url.
	URL string `json:"url,omitempty"`
}

// GitHubRelease is the raw JSON shape of one entry in GitHub's releases API,
// as used by the LLVM project (https://github.com/llvm/llvm-project).
type GitHubRelease struct {
	TagName    string        `json:"tag_name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []GitHubAsset `json:"assets"`
}

// GitHubAsset is a single downloadable file attached to a GitHub release.
type GitHubAsset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"browser_download_url"`
}

type Release struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
	Files   []File `json:"files"`
}

// MachineOS returns the go.dev-style OS identifier for the current machine,
// e.g. "windows", "linux" or "darwin".
func MachineOS() string {
	return runtime.GOOS
}

// MachineArch returns the go.dev-style architecture identifier for the
// current machine, e.g. "amd64" or "arm64".
func MachineArch() string {
	return runtime.GOARCH
}

// FileForMachine returns the downloadable file of the release that matches
// the current machine's OS and architecture. Archives (.zip/.tar.gz) are
// preferred over installers (.msi/.pkg), and source tarballs are ignored.
func (r Release) FileForMachine() (File, bool) {
	var installer *File

	for i := range r.Files {
		f := &r.Files[i]

		if f.OS != runtime.GOOS || f.Arch != runtime.GOARCH {
			continue
		}

		switch f.Kind {
		case "archive":
			return *f, true
		case "installer":
			if installer == nil {
				installer = f
			}
		}
	}

	if installer != nil {
		return *installer, true
	}

	return File{}, false
}

// SizeMB returns the file size in megabytes.
func (f File) SizeMB() float64 {
	return float64(f.Size) / (1024 * 1024)
}
