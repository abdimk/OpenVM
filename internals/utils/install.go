package utils

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
)

type InstallModel struct {
	width  int
	height int
	tools  *ui.ListModel
}

func NewInstallModel() InstallModel {
	items := []ui.Item{
		{TitleText: "C", DescriptionText: "C compiler toolchain (GCC/Clang)"},
		{TitleText: "C++", DescriptionText: "C/C++ compiler toolchain (GCC/Clang)"},
		{TitleText: "Go", DescriptionText: "Go programming language and toolchain"},
		{TitleText: "Python", DescriptionText: "Python interpreter and pip"},
		{TitleText: "Node", DescriptionText: "Node.js runtime and npm"},
		{TitleText: "Rust", DescriptionText: "Rust language and toolchain (rustc, cargo, clippy)"},
		{TitleText: "Docker", DescriptionText: "Docker Engine and CLI"},
		{TitleText: "Kubernetes", DescriptionText: "kubectl command-line tool"},
	}

	l := ui.New("", items, 80, 20)

	return InstallModel{
		tools: l,
	}
}

func (m InstallModel) Init() tea.Cmd {
	if m.tools == nil {
		return nil
	}
	return m.tools.Init()
}

func (m *InstallModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	titleHeight := lipgloss.Height(m.buildListTitle())
	listHeight := height - titleHeight
	if listHeight < 1 {
		listHeight = 1
	}

	if m.tools != nil {
		m.tools.SetSize(width, listHeight)
	}
}

func (m InstallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			if m.tools != nil && m.tools.SettingFilter() {
				return m, m.tools.Update(msg)
			}
			return m, func() tea.Msg { return BackMsg{} }
		case "enter":
			if lang, ok := m.selectedTool(); ok {
				return m, func() tea.Msg { return LanguageSelectedMsg{Language: lang} }
			}
		}
	}

	var cmd tea.Cmd
	if m.tools != nil {
		cmd = m.tools.Update(msg)
	}
	return m, cmd
}

func (m InstallModel) selectedTool() (Language, bool) {
	if m.tools == nil {
		return Language{}, false
	}

	item, ok := m.tools.SelectedItem()
	if !ok {
		return Language{}, false
	}

	for _, lang := range GetAvailable() {
		if strings.EqualFold(lang.Name, item.TitleText) {
			return lang, true
		}
	}

	return Language{Name: item.TitleText}, true
}

func (m InstallModel) buildListTitle() string {
	if m.width <= 0 {
		return "Install Languages and Tools"
	}

	leftBlock := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		PaddingLeft(2).
		PaddingBottom(0).
		Render("Install Languages and Tools")

	rightBlock := lipgloss.NewStyle().
		PaddingRight(2).
		PaddingLeft(1).
		Render(MachineLabel())

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

func (m InstallModel) View() tea.View {
	if m.width <= 0 {
		return tea.NewView("")
	}

	var content string
	if m.tools != nil {
		content = m.tools.View()
	}

	screen := lipgloss.JoinVertical(
		lipgloss.Left,
		m.buildListTitle(),
		content,
	)

	return tea.NewView(screen)
}
