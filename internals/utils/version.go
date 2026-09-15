package utils

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abdimk/openvm/cmd/ui"
)

/*
 * OpenVM: 0.0.1
 * Machine:
 * OpenVM Path: dummy path
 * Installed Packages:
 * Available Storage: 150GB
 * Developer: github.com/abdimk
 */
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

	parts = append(parts,
		" [OpenVM]: "+version,
		" [Machine]: "+GetMachineType(),
		" [OpenVM Path]: dummy path",
		" [Installed Packages]:",
		" [Available Storage]: 150GB",
		" [Developer]: "+developer,
	)

	return strings.Join(parts, "\n\n")
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
