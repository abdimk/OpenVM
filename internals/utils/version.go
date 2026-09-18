package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abdimk/openvm/cmd/ui"
)

const version = "0.0.1"

func NewVersionModel() VersionModel {
	pager := ui.NewPager(
		"OpenVM Version",
		"",
	)
	pager.HideHeader()
	pager.HideFooter()
	return VersionModel{
		pager: pager,
	}
}

func buildVersionContent(width int) string {
	developerURL := "https://github.com/abdimk"

	developer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#58A6FF")).
		Hyperlink(developerURL).
		Render(developerURL)

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A"))
	labelWidth := lipgloss.Width("developer")
	pad := func(label string) string {
		return label + strings.Repeat(" ", labelWidth-lipgloss.Width(label)+1)
	}

	versionValue := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("204")).
		Render(version)

	machineValue := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#D6D6D6")).
		Render(GetMachineType())

	pathValue := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8A8A8A")).
		Render(SwapFilesDir())

	packages := installedToolchains()
	packagesText := "none"
	if len(packages) > 0 {
		packagesText = strings.Join(packages, ", ")
	}
	packagesValue := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFD700")).
		Render(packagesText)

	storageValue := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Render(humanBytes(availableSpaceBytes()))

	row := func(label, value string) string {
		return "  " + labelStyle.Render(pad(label)) + value
	}

	var parts []string

	if logo := ui.OpenVMLogo(width); logo != "" {
		parts = append(parts, logo)
	}

	parts = append(parts, strings.Join([]string{
		row("Version", versionValue),
		row("Machine", machineValue),
		row("Path", pathValue),
		row("Packages", fmt.Sprintf("[ %s ]",packagesValue)),
		row("Storage", storageValue),
		row("Developer", developer),
	}, "\n"))

	return strings.Join(parts, "\n\n")
}

func installedToolchains() []string {
	var names []string
	dir := SwapFilesDir()
	for _, tool := range []string{"go", "python", "node", "llvm", "docker", "kubectl"} {
		if _, err := os.Stat(filepath.Join(dir, tool)); err == nil {
			names = append(names, tool)
		}
	}
	return names
}

func humanBytes(n int64) string {
	if n < 0 {
		return "n/a"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

type VersionModel struct {
	width  int
	height int

	pager ui.Pager
}

func (v VersionModel) Init() tea.Cmd {
	return v.pager.Init()
}

func (v VersionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return v, func() tea.Msg { return BackMsg{} }
		}
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
	}

	v.pager, cmd = v.pager.Update(msg)

	return v, cmd
}

func (v *VersionModel) SetSize(width, height int) {
	v.width = width
	v.height = height

	v.pager.SetContent(buildVersionContent(width))

	v.pager, _ = v.pager.Update(tea.WindowSizeMsg{
		Width:  width,
		Height: height,
	})
}

func (v VersionModel) View() tea.View {
	return tea.NewView(v.pager.View())
}
