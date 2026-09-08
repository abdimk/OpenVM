package utils

import (
	"fmt"
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

	l := ui.New("",items, 80, 20)
	
	
	

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

func (m InstalledModel) buildListTitle() string {
	if m.width <= 0 {
		return "Languages and Tools"
	}

	leftBlock := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		PaddingLeft(2).
		PaddingBottom(0).
		Render("Available Languages")

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
	var content string

	title := m.buildListTitle()
	titleHeight := lipgloss.Height(title)

	if m.languages != nil {
		content = m.languages.View()
	}

	contentHeight := m.height - titleHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	listStyled := lipgloss.NewStyle().
		Height(contentHeight).
		Render(content)

	screen := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		listStyled,
	)

	return tea.NewView(screen)
}