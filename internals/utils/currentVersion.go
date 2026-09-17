package utils

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
)

type CurrentVersionModel struct {
	loading bool
	done    bool
	lines   []string
	pager   ui.Pager
}

func NewCurrentVersionModel() CurrentVersionModel {
	pager := ui.NewPager("Current Version", "")
	pager.ShowPlainHeader("Current Version", MachineLabel())
	return CurrentVersionModel{
		loading: true,
		pager:   pager,
	}
}

type CurrentVersionDoneMsg struct {
	Lines []string
}

func (m CurrentVersionModel) Init() tea.Cmd {
	return func() tea.Msg {
		return CurrentVersionDoneMsg{Lines: currentVersionLines()}
	}
}

func (m CurrentVersionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg { return BackMsg{} }
		}
	case CurrentVersionDoneMsg:
		m.loading = false
		m.done = true
		m.lines = msg.Lines
		m.pager.SetContent(m.renderLines())
	}

	inner, _ := m.pager.Update(msg)
	m.pager = inner

	return m, nil
}

func (m *CurrentVersionModel) SetSize(width, height int) {
	m.pager.SetContent(m.renderLines())

	m.pager, _ = m.pager.Update(tea.WindowSizeMsg{
		Width:  width,
		Height: height,
	})
}

func (m CurrentVersionModel) renderLines() string {
	if !m.done {
		return waitingView()
	}
	return lipgloss.NewStyle().PaddingLeft(2).Render(strings.Join(m.lines, "\n\n"))
}

func (m CurrentVersionModel) View() tea.View {
	return tea.NewView(m.pager.View())
}

func currentVersionLines() []string {
	available := GetAvailable()
	if len(available) == 0 {
		return []string{"No supported languages or tools detected on this machine."}
	}

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	valueStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#D6D6D6"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A"))
	nameStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("204")).
		Background(lipgloss.Color("235"))

	labelWidth := lipgloss.Width("version")
	pad := func(label string) string {
		return label + strings.Repeat(" ", labelWidth-lipgloss.Width(label)+1)
	}

	lines := make([]string, 0, len(available))
	for _, lang := range available {
		version := strings.TrimSpace(lang.Version)
		if version == "" {
			version = "unknown"
		}
		path := lang.Path
		if path == "" {
			path = "(not found)"
		}

		lines = append(lines,
			nameStyle.Render("["+lang.Name+"]")+
				"\n   "+labelStyle.Render(pad("version"))+valueStyle.Render(version)+
				"\n   "+labelStyle.Render(pad("path"))+dimStyle.Render(path),
		)
	}
	return lines
}
