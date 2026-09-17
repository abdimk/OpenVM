package utils

import (
	"runtime"

	"charm.land/lipgloss/v2"
)

func GetMachineType() string {
	switch runtime.GOOS {
	case "linux":
		return "Linux"
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	case "freebsd":
		return "FreeBSD"
	case "openbsd":
		return "OpenBSD"
	default:
		return "Unknown (" + runtime.GOOS + ")"
	}
}

func MachineLabel() string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	keywordStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("204")).
		Background(lipgloss.Color("235"))

	return helpStyle.Render("Machine:") +
		" " +
		helpStyle.Render("[") +
		keywordStyle.Render(GetMachineType()) +
		helpStyle.Render("]")
}
