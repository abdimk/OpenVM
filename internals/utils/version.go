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

	var parts []string

	logo := ui.OpenVMLogo(width)
	if logo != "" {
		parts = append(parts, logo)
	}

	packages := installedToolchains()
	packagesText := "none"
	if len(packages) > 0 {
		packagesText = strings.Join(packages, ", ")
	}

	parts = append(parts,
		" [OpenVM]: "+version,
		" [Machine]: "+GetMachineType(),
		" [OpenVM Path]: "+SwapFilesDir(),
		" [Installed Packages]: "+packagesText,
		" [Available Storage]: "+humanBytes(availableSpaceBytes()),
		" [Developer]: "+developer,
	)

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
