package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	goVersionsURL      = "https://go.dev/dl/?mode=json"
	goAllVersionsURL   = "https://go.dev/dl/?mode=json&include=all"
	requestHTTPTimeout = 15 * time.Second
)

// maxResponseBytes bounds how much of a remote response we buffer, so a
// misbehaving or hostile server cannot exhaust memory.
const maxResponseBytes = 16 << 20

func readAllLimited(r io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return body, nil
}

func FetchStableReleases() ([]Release, error) {
	return fetchReleases(goVersionsURL)
}

func FetchAllReleases() ([]Release, error) {
	return fetchReleases(goAllVersionsURL)
}

func ReleasesForMachine() ([]Release, error) {
	releases, err := FetchAllReleases()
	if err != nil {
		return nil, err
	}

	var out []Release
	for _, r := range releases {
		if _, ok := r.FileForMachine(); ok {
			out = append(out, r)
		}
	}

	return out, nil
}

// ResolveFileSize fills File.Size from the artifact's Content-Length when it is
// not already known. Best effort: leaves Size untouched on failure or when the
// server does not report a length.
func ResolveFileSize(file *File) error {
	if file == nil || file.Size > 0 || file.URL == "" {
		return nil
	}

	client := &http.Client{Timeout: requestHTTPTimeout}
	resp, err := client.Head(file.URL)
	if err != nil {
		return fmt.Errorf("resolving size for %s: %w", file.Filename, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resolving size for %s: unexpected status %s", file.Filename, resp.Status)
	}

	if resp.ContentLength > 0 {
		file.Size = resp.ContentLength
	}
	return nil
}

var sha256HexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)


func fetchSHA256Sidecar(client *http.Client, artifactURL string) string {
	resp, err := client.Get(artifactURL + ".sha256")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	if err != nil {
		return ""
	}
	for _, field := range strings.Fields(string(body)) {
		lower := strings.ToLower(field)
		if sha256HexRe.MatchString(lower) {
			return lower
		}
	}
	return ""
}


func ResolveChecksum(file *File) {
	if file == nil || file.SHA256 != "" || file.URL == "" {
		return
	}
	client := &http.Client{Timeout: requestHTTPTimeout}
	file.SHA256 = fetchSHA256Sidecar(client, file.URL)
}

func fetchReleases(url string) ([]Release, error) {
	client := &http.Client{Timeout: requestHTTPTimeout}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching Go versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching Go versions: unexpected status %s", resp.Status)
	}

	body, err := readAllLimited(resp.Body, maxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("reading Go versions response: %w", err)
	}

	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("parsing Go versions response: %w", err)
	}

	return releases, nil
}
