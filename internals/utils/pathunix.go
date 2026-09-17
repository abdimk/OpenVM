package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func activatePath(dir string) error {
	if runtime.GOOS == "windows" {
		if err := ensureWindowsPath(dir); err != nil {
			return err
		}
		prependProcessPath(dir)
		return nil
	}
	if err := ensureUnixPath(dir); err != nil {
		return err
	}
	prependProcessPath(dir)
	return nil
}

func prependProcessPath(dir string) {
	sep := string(os.PathListSeparator)
	cur := os.Getenv("PATH")
	if cur == "" {
		_ = os.Setenv("PATH", dir)
		return
	}
	for _, p := range filepath.SplitList(cur) {
		if runtime.GOOS == "windows" {
			if strings.EqualFold(p, dir) {
				return
			}
		} else if p == dir {
			return
		}
	}
	_ = os.Setenv("PATH", dir+sep+cur)
}

func ensureUnixPath(dir string) error {
	rc, line, err := unixShellConfig(dir)
	if err != nil {
		return err
	}

	existing, readErr := os.ReadFile(rc)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("adding %s to %s: %w", dir, rc, readErr)
	}
	if strings.Contains(string(existing), dir) {
		return nil
	}

	content := strings.TrimRight(string(existing), "\n")
	if content != "" {
		content += "\n"
	}
	content += line + "\n"

	if err := os.WriteFile(rc, []byte(content), 0o644); err != nil {
		return fmt.Errorf("adding %s to %s: %w", dir, rc, err)
	}
	return nil
}

func unixShellConfig(dir string) (rc, line string, err error) {
	home, userErr := os.UserHomeDir()
	if userErr != nil {
		return "", "", fmt.Errorf("locating home directory: %w", userErr)
	}

	quoted := "'" + strings.ReplaceAll(dir, "'", `'\''`) + "'"
	switch strings.ToLower(filepath.Base(os.Getenv("SHELL"))) {
	case "zsh":
		return filepath.Join(home, ".zshrc"), "export PATH=" + quoted + ":$PATH", nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), "fish_add_path --prepend " + quoted, nil
	case "bash":
		return filepath.Join(home, ".bashrc"), "export PATH=" + quoted + ":$PATH", nil
	default:
		return filepath.Join(home, ".profile"), "export PATH=" + quoted + ":$PATH", nil
	}
}
