package utils

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
)

type BackMsg struct{}

type LanguageSelectedMsg struct {
	Language Language
}

type UninstalledMsg struct {
	Name string
	Err  error
}

type UninstallDoneResetMsg struct{}

type InstalledModel struct {
	width      int
	height     int
	languages  *ui.ListModel
	langs      []Language
	confirming bool
	confirmYes bool
}

func NewInstalledModel() InstalledModel {
	langs := GetLanguages()

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
	case UninstalledMsg:
		m.confirming = false
		if msg.Err != nil {
			EmitFooterHintHighlighted(fmt.Sprintf("Uninstall failed: %v • esc back", msg.Err), "#ff5555", "#444444")
			return m, m.refresh()
		}
		EmitFooterHintHighlighted(fmt.Sprintf("Uninstall complete: %s", msg.Name), "#00ff00", "#444444")
		return m, tea.Batch(
			m.refresh(),
			tea.Tick(3*time.Second, func(time.Time) tea.Msg { return UninstallDoneResetMsg{} }),
		)

	case UninstallDoneResetMsg:
		ClearFooterHint()
		return m, nil

	case tea.KeyMsg:
		if m.confirming {
			switch msg.String() {
			case "left":
				m.confirmYes = true
				return m, nil
			case "right":
				m.confirmYes = false
				return m, nil
			case "esc", "backspace":
				m.confirming = false
				return m, nil
			case "enter":
				if m.confirmYes {
					if lang, ok := m.selectedLanguage(); ok {
						m.confirming = false
						return m, m.uninstallCmd(lang.Name)
					}
				}
				m.confirming = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "esc", "backspace":
			if m.languages != nil && m.languages.SettingFilter() {
				return m, m.languages.Update(msg)
			}
			return m, func() tea.Msg { return BackMsg{} }
		case "u":
			if _, ok := m.selectedLanguage(); ok {
				m.confirming = true
				m.confirmYes = true
				return m, nil
			}
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

func (m InstalledModel) uninstallCmd(name string) tea.Cmd {
	EmitFooterHintHighlighted(fmt.Sprintf("Uninstalling %s...", name), "#FFA500", "#444444")
	return func() tea.Msg {
		return UninstalledMsg{Name: name, Err: UninstallTool(name)}
	}
}

func (m *InstalledModel) refresh() tea.Cmd {
	m.langs = GetLanguages()

	items := make([]ui.Item, 0, len(m.langs))
	for _, lang := range m.langs {
		items = append(items, ui.Item{
			TitleText:       lang.Name,
			DescriptionText: strings.TrimSpace(lang.Version),
		})
	}

	if m.languages != nil {
		return m.languages.SetItems(items)
	}
	return nil
}

func (m *InstalledModel) Refresh() tea.Cmd {
	return m.refresh()
}

func (m InstalledModel) Confirming() bool {
	return m.confirming
}

func (m InstalledModel) ConfirmationView() string {
	name := ""
	if lang, ok := m.selectedLanguage(); ok {
		name = lang.Name
	}

	gray := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	green := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ff00"))
	pink := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("204"))
	pinkBg := pink.Background(lipgloss.Color("235"))
	dim := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("241"))

	yes := dim.Render("Yes")
	no := dim.Render("No")
	if m.confirmYes {
		yes = green.Render("Yes")
	} else {
		no = pinkBg.Render("No")
	}

	bracket := func(s string) string {
		return gray.Render("[") + s + gray.Render("]")
	}

	return bracket(yes) + gray.Render(" | ") + bracket(no) +
		"  " +
		pink.Render(fmt.Sprintf("Are you sure you wanted to remove %s ?", name)) +
		"   " +
		gray.Render("←/→ choose • enter confirm • esc back")
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

func (m InstalledModel) View() tea.View {
	if m.width <= 0 {
		return tea.NewView("")
	}

	var content string
	if m.languages != nil {
		content = m.languages.View()
	}

	screen := lipgloss.JoinVertical(
		lipgloss.Left,
		m.buildListTitle(),
		content,
	)

	return tea.NewView(screen)
}
