package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	goVersionsURL      = "https://go.dev/dl/?mode=json"
	goAllVersionsURL   = "https://go.dev/dl/?mode=json&include=all"
	requestHTTPTimeout = 15 * time.Second
)

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading Go versions response: %w", err)
	}

	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("parsing Go versions response: %w", err)
	}

	return releases, nil
}
