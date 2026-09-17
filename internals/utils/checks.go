package utils

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func firstSemverField(raw string) string {
	for _, f := range strings.Fields(raw) {
		trimmed := strings.Trim(f, ",;:()[]{}<>\"'")
		if strings.HasPrefix(trimmed, "v") && len(trimmed) > 1 {
			trimmed = trimmed[1:]
		}
		if strings.HasPrefix(trimmed, "go") && len(trimmed) > 2 {
			trimmed = trimmed[2:]
		}
		for _, sep := range []string{"+", "-"} {
			if i := strings.Index(trimmed, sep); i > 0 {
				trimmed = trimmed[:i]
			}
		}
		if looksLikeSemver(trimmed) {
			return trimmed
		}
	}
	return ""
}

func looksLikeSemver(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

func semverGreater(a, b string) bool {
	pa, errA := semverParts(a)
	pb, errB := semverParts(b)
	if errA != nil || errB != nil {
		return false
	}
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return len(pa) > len(pb)
}

func semverParts(s string) ([]int, error) {
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func probeURL(url string) error {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Head(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}

func pathContainsDir(dir string) bool {
	if runtime.GOOS == "windows" {
		for _, p := range filepath.SplitList(os.Getenv("PATH")) {
			if strings.EqualFold(strings.TrimRight(p, `\`), strings.TrimRight(dir, `\`)) {
				return true
			}
		}
		return false
	}

	rc, _, err := unixShellConfig(dir)
	if err != nil {
		return false
	}
	b, err := os.ReadFile(rc)
	if err != nil {
		return false
	}
	return strings.Contains(string(b), dir)
}

func toolPathDir(toolDir string) string {
	switch toolDir {
	case "go", "llvm", "rust":
		return filepath.Join(toolDir, "bin")
	case "python", "node":
		if runtime.GOOS != "windows" {
			return filepath.Join(toolDir, "bin")
		}
		return toolDir
	default:
		return toolDir
	}
}
