package utils

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
)

type BackMsg struct{}

type LanguageSelectedMsg struct {
	Language Language
}

type InstalledModel struct {
	width     int
	height    int
	languages *ui.ListModel
	langs     []Language
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

	l := ui.New("", items, 80, 20)

	return InstalledModel{
		languages: l,
		langs:     langs,
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

	// The title row is drawn on top of the list in View(), so reserve its
	// height here. If the list is told the full height, title + list =
	// height + 1 and the last row (footer bar) gets clipped off-screen.
	titleHeight := lipgloss.Height(m.buildListTitle())
	listHeight := height - titleHeight
	if listHeight < 1 {
		listHeight = 1
	}

	if m.languages != nil {
		m.languages.SetSize(width, listHeight)
	}
}

func (m InstalledModel) selectedLanguage() (Language, bool) {
	item, ok := m.languages.SelectedItem()
	if !ok {
		return Language{}, false
	}

	for _, lang := range m.langs {
		if lang.Name == item.TitleText {
			return lang, true
		}
	}
	return Language{}, false
}

func (m InstalledModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg { return BackMsg{} }
		case "enter":
			if lang, ok := m.selectedLanguage(); ok {
				return m, func() tea.Msg { return LanguageSelectedMsg{Language: lang} }
			}
		}
	}

	var cmd tea.Cmd
	if m.languages != nil {
		cmd = m.languages.Update(msg)
	}
	return m, cmd
}

func (m InstalledModel) buildListTitle() string {
	if m.width <= 0 {
		return "Languages and Tools"
	}

	leftBlock := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		PaddingLeft(2).
		PaddingBottom(0).
		Render("Current Version of Languages and Dev Tools")

	rightBlock := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		PaddingRight(2).
		PaddingLeft(1).
		Render(fmt.Sprintf("Machine: [%s]", GetMachineType()))

	leftW := lipgloss.Width(leftBlock)
	rightW := lipgloss.Width(rightBlock)

	gap := m.width - leftW - rightW
	if gap < 0 {
		gap = 0
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftBlock,
		strings.Repeat(" ", gap),
		rightBlock,
	)
}

func (m InstalledModel) View() tea.View {
	if m.width <= 0 {
		return tea.NewView("")
	}

	var content string
	if m.languages != nil {
		content = m.languages.View()
	}

	// The list is already sized to exactly height - titleHeight (see
	// SetSize), so render it as-is — no Height() wrapper. lipgloss Height()
	// is a minimum, not a crop, so it could only ever grow the view.
	screen := lipgloss.JoinVertical(
		lipgloss.Left,
		m.buildListTitle(),
		content,
	)

	return tea.NewView(screen)
}
