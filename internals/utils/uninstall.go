package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func UninstallTool(name string) error {
	dir, ok := nameToToolDir(name)
	if !ok {
		return fmt.Errorf("no uninstall plan for %s", name)
	}

	root := SwapFilesDir()
	binDir := filepath.Join(root, toolPathDir(dir))

	if err := removeFromPath(binDir); err != nil {
		return fmt.Errorf("removing %s from PATH: %w", binDir, err)
	}

	if err := os.RemoveAll(filepath.Join(root, dir)); err != nil {
		return fmt.Errorf("removing %s installation: %w", dir, err)
	}
	os.RemoveAll(filepath.Join(root, dir+".bak"))

	removeToolCache(root, name)

	return nil
}

func nameToToolDir(name string) (string, bool) {
	switch name {
	case Go:
		return "go", true
	case Python:
		return "python", true
	case Node:
		return "node", true
	case Rust:
		return "rust", true
	case Gpp, Gcc:
		return "llvm", true
	case Docker:
		return "docker", true
	case Kubernetes:
		return "kubectl", true
	}
	return "", false
}

func removeFromPath(dir string) error {
	if runtime.GOOS == "windows" {
		return removeWindowsPathEntry(dir)
	}
	return removeUnixPathEntry(dir)
}

func removeWindowsPathEntry(dir string) error {
	script := fmt.Sprintf(
		"$p=[Environment]::GetEnvironmentVariable('Path','User'); $needle='%s'; $new=($p -split ';' | Where-Object { $_ -and ($_.TrimEnd('\\') -ine $needle) }) -join ';'; if($p -ne $new){[Environment]::SetEnvironmentVariable('Path',$new,'User')}",
		strings.ReplaceAll(dir, "'", "''"),
	)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("removing %s from user PATH: %w: %s", dir, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func removeUnixPathEntry(dir string) error {
	rc, _, err := unixShellConfig(dir)
	if err != nil {
		return err
	}

	b, err := os.ReadFile(rc)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", rc, err)
	}

	lines := strings.Split(string(b), "\n")
	kept := lines[:0]
	for _, l := range lines {
		if strings.Contains(l, dir) {
			continue
		}
		kept = append(kept, l)
	}

	return os.WriteFile(rc, []byte(strings.TrimRight(strings.Join(kept, "\n"), "\n")), 0o644)
}

var toolCachePrefixes = map[string][]string{
	Go:         {"go1."},
	Python:     {"python-", "cpython-"},
	Node:       {"node-"},
	Rust:       {"rust-"},
	Gpp:        {"LLVM-", "clang+llvm-"},
	Gcc:        {"LLVM-", "clang+llvm-"},
	Docker:     {"docker-"},
	Kubernetes: {"kubectl"},
}

func removeToolCache(root, name string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		for _, p := range toolCachePrefixes[name] {
			if strings.HasPrefix(e.Name(), p) {
				_ = os.Remove(filepath.Join(root, e.Name()))
				break
			}
		}
	}
}
