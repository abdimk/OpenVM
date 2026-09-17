package api

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var rustChannelURL = "https://static.rust-lang.org/dist/channel-rust-stable.toml"

const rustHTTPTimeout = 60 * time.Second

func rustTargetTriple(targetOS, targetArch string) (string, bool) {
	switch targetOS + "/" + targetArch {
	case "windows/amd64":
		return "x86_64-pc-windows-msvc", true
	case "windows/arm64":
		return "aarch64-pc-windows-msvc", true
	case "linux/amd64":
		return "x86_64-unknown-linux-gnu", true
	case "linux/arm64":
		return "aarch64-unknown-linux-gnu", true
	case "darwin/amd64":
		return "x86_64-apple-darwin", true
	case "darwin/arm64":
		return "aarch64-apple-darwin", true
	default:
		return "", false
	}
}

func FetchRustReleases() ([]Release, error) {
	file, err := FetchRustArtifact("")
	if err != nil {
		return nil, err
	}
	return []Release{{
		Version: file.Version,
		Stable:  true,
		Files:   []File{file},
	}}, nil
}

func FetchRustArtifact(version string) (File, error) {
	triple, ok := rustTargetTriple(runtime.GOOS, runtime.GOARCH)
	if !ok {
		return File{}, fmt.Errorf("no official Rust toolchain for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	file, err := fetchRustArtifactForTriple(triple)
	if err != nil {
		return File{}, err
	}

	if version != "" && file.Version != version {
		return File{}, fmt.Errorf("Rust %s is no longer the current stable release; latest is %s", version, file.Version)
	}
	return file, nil
}

func fetchRustArtifactForTriple(triple string) (File, error) {
	client := &http.Client{Timeout: rustHTTPTimeout}

	resp, err := client.Get(rustChannelURL)
	if err != nil {
		return File{}, fmt.Errorf("fetching Rust releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return File{}, fmt.Errorf("fetching Rust releases: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return File{}, fmt.Errorf("reading Rust releases response: %w", err)
	}

	m, err := parseRustManifest(body, triple)
	if err != nil {
		return File{}, err
	}
	if m.url == "" {
		return File{}, fmt.Errorf("no official Rust toolchain for %s", triple)
	}

	return File{
		Filename: filenameFromURL(m.url),
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Version:  m.version,
		SHA256:   m.sha256,
		Kind:     "archive",
		URL:      m.url,
	}, nil
}

type rustManifestTarget struct {
	version string
	url     string
	sha256  string
}

func parseRustManifest(body []byte, triple string) (*rustManifestTarget, error) {
	section := ""
	manifestVersion := ""
	rustVersion := ""
	url := ""
	urlXZ := ""
	hash := ""
	hashXZ := ""

	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = trimmed
			continue
		}

		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)

		if section == "[manifest]" && key == "version" {
			if v, er := strconv.Unquote(strings.TrimSpace(value)); er == nil {
				manifestVersion = v
			}
			continue
		}

		if section == "[pkg.rust]" && key == "version" {
			if v, er := strconv.Unquote(strings.TrimSpace(value)); er == nil {
				rustVersion = strings.Fields(v)[0]
			}
			continue
		}

		if section != "[pkg.rust.target."+triple+"]" {
			continue
		}

		val, er := strconv.Unquote(strings.TrimSpace(value))
		if er != nil {
			continue
		}
		switch key {
		case "url":
			url = val
		case "hash":
			hash = val
		case "xz_url":
			urlXZ = val
		case "xz_hash":
			hashXZ = val
		}
	}

	version := manifestVersion
	if rustVersion != "" {
		version = rustVersion
	}
	if version == "" {
		return nil, fmt.Errorf("parsing Rust channel manifest: missing version")
	}
	if url == "" {
		return nil, fmt.Errorf("parsing Rust channel manifest: no artifact URL for %s", triple)
	}

	downloadURL := url
	downloadHash := hash
	if urlXZ != "" {
		downloadURL = urlXZ
		downloadHash = hashXZ
	}

	return &rustManifestTarget{
		version: version,
		url:     downloadURL,
		sha256:  downloadHash,
	}, nil
}

func filenameFromURL(raw string) string {
	if i := strings.LastIndex(raw, "/"); i >= 0 && i+1 < len(raw) {
		return raw[i+1:]
	}
	return raw
}
