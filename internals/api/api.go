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

// FetchStableReleases returns the currently stable Go releases from
// https://go.dev/dl/?mode=json
func FetchStableReleases() ([]Release, error) {
	return fetchReleases(goVersionsURL)
}

// FetchAllReleases returns every archived Go release from
// https://go.dev/dl/?mode=json&include=all
func FetchAllReleases() ([]Release, error) {
	return fetchReleases(goAllVersionsURL)
}

// ReleasesForMachine fetches all Go releases and keeps only the ones that
// ship a downloadable file matching the current machine's OS/arch.
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
