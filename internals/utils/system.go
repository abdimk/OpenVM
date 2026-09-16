package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	Go     = "Go"
	Python = "Python"
	Node   = "Node"
	Rust   = "Rust"
	Gpp    = "C++"
	Gcc    = "C"
)

const (
	Docker     = "Docker"
	Kubernetes = "Kubernetes"
)

var languageBinaries = map[string][]string{
	Go:     {"go"},
	Python: pythonCandidates(),
	Node:   {"node"},
	Rust:   {"rustc"},
	Gpp:    {"g++", "clang++"},
	Gcc:    {"gcc", "clang"},
}

var toolBinaries = map[string][]string{
	Docker:     {"docker"},
	Kubernetes: {"kubectl"},
}

type Language struct {
	Name    string
	Path    string
	Version string
}

func pythonCandidates() []string {
	if runtime.GOOS != "windows" {
		return []string{"python3", "python"}
	}
	var candidates []string
	if p := windowsPythonOnPath(); p != "" {
		candidates = append(candidates, p)
	}
	if p := windowsStorePython(); p != "" {
		candidates = append(candidates, p)
	}
	return append(candidates, "py")
}

func windowsPythonOnPath() string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, "python.exe")
		info, err := os.Stat(p)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			continue
		}
		return p
	}
	return ""
}

func windowsStorePython() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return ""
	}
	p := filepath.Join(localAppData, "Microsoft", "WindowsApps", "python.exe")
	info, err := os.Stat(p)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return ""
	}
	return p
}

func findBinaries(names []string, binaries map[string][]string) []Language {
	if os.Getenv("PATH") == "" {
		return []Language{}
	}

	var found []Language
	alreadyFound := make(map[string]bool)

	for _, name := range names {
		for _, binary := range binaries[name] {
			if alreadyFound[name] {
				break
			}

			path, err := exec.LookPath(binary)
			if err != nil {
				continue
			}

			found = append(found, Language{
				Name:    name,
				Path:    path,
				Version: getVersion(binary),
			})
			alreadyFound[name] = true
			break
		}
	}

	return found
}

func GetLanguages() []Language {
	return findBinaries(
		[]string{Go, Python, Node, Rust, Gpp, Gcc},
		languageBinaries,
	)
}

func GetTools() []Language {
	return findBinaries(
		[]string{Docker, Kubernetes},
		toolBinaries,
	)
}

func GetAvailable() []Language {
	return append(GetLanguages(), GetTools()...)
}

func getVersion(binary string) string {
	var args []string

	switch binary {
	case "go", "rustc":
		args = []string{"version"}
	case "python3", "python", "python.exe", "py":
		args = []string{"--version"}
	case "node":
		args = []string{"--version"}
	case "g++", "gcc", "clang++", "clang":
		args = []string{"--version"}
	default:
		args = []string{"--version"}
	}

	out, err := exec.Command(binary, args...).CombinedOutput()
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}

	return "unknown"
}
