package utils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DownloadPhase tells the UI which stage of the download+install pipeline a
// progress event belongs to.
type DownloadPhase int

const (
	PhaseIdle DownloadPhase = iota
	PhaseDownloading
	PhaseVerifying
	PhaseExtracting
	PhaseSwapping
	PhaseDone
	PhaseFailed
	PhaseCancelled
)

// DownloadProgressMsg is emitted while a download is running. Percent is in
// the 0..1 range and is -1 while the total size is unknown. Seq tags the
// pipeline instance so the UI can ignore events from stale runs.
type DownloadProgressMsg struct {
	Seq        int
	Phase      DownloadPhase
	Downloaded int64
	Total      int64
	Percent    float64
	File       string
	Err        error
}

// DownloadResultMsg is emitted once when the whole pipeline finishes.
type DownloadResultMsg struct {
	Seq       int
	Phase     DownloadPhase
	Version   string
	Filename  string
	Archive   string // path of the downloaded archive on disk
	Err       error
	Duration  time.Duration
	BytesRead int64
}

const downloadHTTPTimeout = 10 * time.Minute

// DownloadFile streams url to destPath (written via .part then renamed so an
// interrupted download never leaves a half-written file at destPath),
// reporting progress through progressCh and aborting when ctx is cancelled.
// If wantSHA256 is non-empty the downloaded file is verified against it and
// the function fails on mismatch. seq is stamped into every progress event
// so the UI can discard events from superseded pipeline runs.
func DownloadFile(ctx context.Context, url, destPath, wantSHA256 string, seq int, progressCh chan<- DownloadProgressMsg) error {
	ctx, cancel := context.WithTimeout(ctx, downloadHTTPTimeout)
	defer cancel()

	partPath := destPath + ".part"

	client := &http.Client{Timeout: downloadHTTPTimeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: unexpected status %s", url, resp.Status)
	}

	total := resp.ContentLength

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creating download directory: %w", err)
	}

	out, err := os.Create(partPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	hasher := sha256.New()
	buf := make([]byte, 256*1024)
	var downloaded int64

	// Clean up the .part file on any failure path.
	defer func() {
		if err != nil {
			out.Close()
			os.Remove(partPath)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			err = fmt.Errorf("download cancelled")
			return err
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			downloaded += int64(n)

			if _, werr := out.Write(buf[:n]); werr != nil {
				err = fmt.Errorf("writing download: %w", werr)
				return err
			}
			hasher.Write(buf[:n])

			if progressCh != nil && (total <= 0 || downloaded*100/total != (downloaded-int64(n))*100/total || downloaded == total) {
				pct := -1.0
				if total > 0 {
					pct = float64(downloaded) / float64(total)
				}
				// Non-blocking: a cancelled run must never block forever on a
				// channel nobody drains anymore. Dropped events are harmless —
				// the final state arrives via DownloadResultMsg.
				select {
				case progressCh <- DownloadProgressMsg{
					Seq:        seq,
					Phase:      PhaseDownloading,
					Downloaded: downloaded,
					Total:      total,
					Percent:    pct,
					File:       filepath.Base(destPath),
				}:
				default:
				}
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			err = fmt.Errorf("reading download: %w", readErr)
			return err
		}
	}

	if err = out.Sync(); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}

	// Checksum verification before the file is considered good.
	if progressCh != nil {
		select {
		case progressCh <- DownloadProgressMsg{Seq: seq, Phase: PhaseVerifying, File: filepath.Base(destPath)}:
		default:
		}
	}

	if wantSHA256 != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if got != wantSHA256 {
			err = fmt.Errorf("checksum mismatch: expected %s, got %s", wantSHA256, got)
			return err
		}
	}

	if err = os.Rename(partPath, destPath); err != nil {
		return fmt.Errorf("finalizing download: %w", err)
	}

	return nil
}
