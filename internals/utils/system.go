package utils

import (
	"os"
	"os/exec"
	"strings"
)

const ( 
	Go = "Go"
	Python = "Python"
	Node = "Node"
	Rust = "Rust"
	Gpp = "C++"
	Gcc = "C"
)

var languageBinaries = map[string][]string{
	Go: {"go"},
	Python: {"python3", "python"},
	Node: {"node"},
	Rust: {"rustc"},
	Gpp: {"g++"},
	Gcc: {"gcc"},
}


type Language struct{
	Name string
	Path string
	Version string
	
}




func GetLanguages()[]Language{
	if GetMachineType() != "Linux"{
		return []Language{}
	}
	
	if os.Getenv("PATH") == ""{
		return []Language{}
	}
	
	var found []Language
	
	alreadyFound := make(map[string]bool)
	

		for _, langName := range []string{Go, Python, Node, Rust, Gpp, Gcc} {
			binaries := languageBinaries[langName]

			for _, binary := range binaries {

				if alreadyFound[langName] {
					break
				}


				path, err := exec.LookPath(binary)
				if err != nil {
					continue
				}

				version := getVersion(binary)

				found = append(found, Language{
					Name:    langName,
					Path:    path,
					Version: version,
				})
				alreadyFound[langName] = true
				break
			}
		}

		return found
}

func getVersion(binary string) string {
	var args []string

	switch binary {
	case "go", "rustc":
		args = []string{"version"}
	case "python3", "python":
		args = []string{"--version"}
	case "node":
		args = []string{"--version"}
	case "g++", "gcc":
		args = []string{"--version"}
	default:
		args = []string{"--version"}
	}

	out, err := exec.Command(binary, args...).CombinedOutput()
	if err != nil {
		return "unknown"
	}

	// Take only the first line
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}

	return "unknown"
}