package utils

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
)

type BackMsg struct{}

type InstalledModel struct {
	width  int
	height int

	languages *ui.ListModel
}

func NewInstalledModel() InstalledModel {
	langs := GetLanguages() // same package, no import needed

	items := make([]ui.Item, 0, len(langs))
	for _, lang := range langs {
		items = append(items, ui.Item{
			TitleText:       lang.Name,
			DescriptionText: strings.TrimSpace(lang.Version),
		})
	}

	l := ui.New("Installed Languages", items, 80, 20)
	l.SetShowTitle(true)
	
	
	

	return InstalledModel{
		languages: l,
	}
}

func (m InstalledModel) Init() tea.Cmd {
	if m.languages == nil {
		return nil
	}
	return m.languages.Init()
}

func (m *InstalledModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	if m.languages != nil {
		m.languages.SetSize(width, height)
	}
}

func (m InstalledModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.languages != nil {
			m.languages.SetSize(m.width, m.height)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg { return BackMsg{} }
		}
	}

	var cmd tea.Cmd
	if m.languages != nil {
		cmd = m.languages.Update(msg)
	}
	return m, cmd
}

func (m InstalledModel) View() tea.View {
	var content string

	if m.languages != nil {
		content = m.languages.View()
	}

	if m.height > 0 {
		pad := m.height - lipgloss.Height(content)
		if pad > 0 {
			content += strings.Repeat("\n", pad)
		}
	}

	return tea.NewView(content)
}