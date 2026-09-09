package process

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abdimk/openvm/internals/utils"
)

type SelectedInstalledMsg struct {
	Language utils.Language
}

type SelectedInstalledModel struct {
	language utils.Language
	width    int
	height   int
}

func NewSelectedInstalledModel(lang utils.Language) SelectedInstalledModel {
	return SelectedInstalledModel{
		language: lang,
	}
}

func (m SelectedInstalledModel) Init() tea.Cmd {
	return nil
}

func (m *SelectedInstalledModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m SelectedInstalledModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg { return utils.BackMsg{} }
		}
	}
	return m, nil
}

func (m SelectedInstalledModel) buildHeader() string {
	if m.width <= 0 {
		return fmt.Sprintf("Language: [%s]", m.language.Name)
	}

	leftBlock := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		PaddingLeft(2).
		PaddingBottom(0).
		Render("Language:")

	greenStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ff00")).
		Render(fmt.Sprintf(" [%s]", m.language.Name))

	rightBlock := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		PaddingRight(2).
		PaddingLeft(1).
		Render(fmt.Sprintf("Machine: [%s]", utils.GetMachineType()))

	leftW := lipgloss.Width(leftBlock) + lipgloss.Width(greenStyle)
	rightW := lipgloss.Width(rightBlock)

	gap := m.width - leftW - rightW
	if gap < 0 {
		gap = 0
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftBlock,
		greenStyle,
		strings.Repeat(" ", gap),
		rightBlock,
	)
}

func (m SelectedInstalledModel) View() tea.View {
	var content strings.Builder


	content.WriteString(m.buildHeader())
	content.WriteString("\n\n")

	body := lipgloss.NewStyle().PaddingLeft(2).Render(
		fmt.Sprintf("Name: %s\nPath: %s\nVersion: %s",
			m.language.Name, m.language.Path, m.language.Version),
	)
	content.WriteString(body)

	screen := content.String()

	if m.height > 0 {
		pad := m.height - lipgloss.Height(screen)
		if pad > 0 {
			screen += strings.Repeat("\n", pad)
		}
	}

	return tea.NewView(screen)
}
