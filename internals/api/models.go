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

	URL string `json:"url,omitempty"`
}

type GitHubRelease struct {
	TagName    string        `json:"tag_name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []GitHubAsset `json:"assets"`
}

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

func MachineOS() string {
	return runtime.GOOS
}

func MachineArch() string {
	return runtime.GOARCH
}

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

func (f File) SizeMB() float64 {
	return float64(f.Size) / (1024 * 1024)
}
