package utils

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// InfoModel is a simple reusable screen that renders a title and a body,
// padded to fill the available height so the shared footer stays pinned
// to the bottom of the screen.
type InfoModel struct {
	title  string
	body   string
	width  int
	height int
}

func NewInfoModel(title, body string) InfoModel {
	return InfoModel{
		title: title,
		body:  body,
	}
}

func (m InfoModel) Init() tea.Cmd {
	return nil
}

func (m InfoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg { return BackMsg{} }
		}
	}
	return m, nil
}

func (m *InfoModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m InfoModel) View() tea.View {
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Render(m.title))
	b.WriteString("\n\n")

	if m.body != "" {
		b.WriteString(m.body)
		b.WriteString("\n\n")
	}

	content := b.String()
	if m.height > 0 {
		pad := m.height - lipgloss.Height(content)
		if pad > 0 {
			content += strings.Repeat("\n", pad)
		}
	}

	return tea.NewView(lipgloss.NewStyle().PaddingLeft(2).Render(content))
}
