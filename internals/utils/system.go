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
)

var languageBinaries = map[string][]string{
	Go: {"go"},
	Python: {"python3", "python"},
	Node: {"node"},
	Rust: {"rustc"},
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
	

		for _, langName := range []string{Go, Python, Node, Rust} {
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
		default:
			args = []string{"--version"}
		}

		
		out, err := exec.Command(binary, args...).CombinedOutput()
		if err != nil {
			return "unknown"
		}

		return strings.TrimSpace(string(out))
}


