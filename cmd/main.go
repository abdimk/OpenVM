package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
	"github.com/abdimk/openvm/internals/utils"
)

type Screen int

const (
	MainMenuScreen Screen = iota
	InstalledScreen
	Version
)

type model struct {
	width          int
	height         int
	loadingSpinner ui.Spinner
	list           *ui.ListModel
	table          *ui.TableModel
	packageManager *ui.PackageManagerModel

	screen    Screen
	installed utils.InstalledModel
	version   utils.VersionModel
}

func newModel() *model {
	items := []ui.Item{
		{TitleText: "Install", DescriptionText: "Install a programming language or development tool"},
		{TitleText: "Installed", DescriptionText: "List all installed languages and tools"},
		{TitleText: "Check For Update", DescriptionText: "Check for and update installed tools"},
		{TitleText: "Repair", DescriptionText: "Detect and repair broken tool installations"},
		{TitleText: "Doctor", DescriptionText: "Check your environment and diagnose configuration issues"},
		{TitleText: "Completion", DescriptionText: "Generate shell completion scripts"},
		{TitleText: "Version", DescriptionText: "Show OpenVM version information"},
		{TitleText: "Exit", DescriptionText: "Exit OpenVM"},
	}

	packages := []string{
		"auth-service",
		"receiver",
		"mock-data",
		"synheart-cli",
	}

	return &model{
		loadingSpinner: ui.SpinnerModel("Loading..."),
		list:           ui.New("Available Commands", items, 80, 20),
		packageManager: ui.NewPackageManager(
			packages,
			installPackage,
		),
		screen:    MainMenuScreen,
		installed: utils.NewInstalledModel(),
		version:   utils.NewVersionModel(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.loadingSpinner.Init(),
		m.packageManager.Init(),
	)
}

func (m model) header() string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#646e78")).
		Width(m.width).
		Align(lipgloss.Center).
		MarginBottom(1)

	return style.Render("OpenVM")
}

func (m model) description() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Width(m.width).
		Align(lipgloss.Center).
		MarginBottom(1)

	return style.Render(
		"OpenVM is a Terminal User Interface (TUI) for managing development tools and programming languages.\n" +
			"It simplifies installing, updating, switching between versions, and repairing tools from one unified terminal interface.",
	)
}
func (m model) buildListTitle() string {
	if m.width <= 0 {
		return "Available Commands"
	}

	// Left: bold, white text, default background, slightly indented
	leftBlock := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		PaddingLeft(2).
		PaddingBottom(0).
		Render("Available Commands")

	rightBlock := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		// Border(lipgloss.RoundedBorder()).
		PaddingRight(2).
		PaddingLeft(1).
		Render(fmt.Sprintf("Machine: [%s]", utils.GetMachineType()))
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
func (m *model) updateLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	headerHeight := lipgloss.Height(m.header())
	descriptionHeight := lipgloss.Height(m.description())

	footer := ui.NewFooter(
		m.width,
		m.footerText(),
	).View()

	footerHeight := lipgloss.Height(footer)

	contentHeight := m.height -
		headerHeight -
		descriptionHeight -
		footerHeight

	if contentHeight < 1 {
		contentHeight = 1
	}

	switch m.screen {
	case MainMenuScreen:
		titleHeight := lipgloss.Height(m.buildListTitle())

		listHeight := contentHeight - titleHeight

		if listHeight < 1 {
			listHeight = 1
		}

		m.list.SetSize(m.width, listHeight)

	case InstalledScreen:
		m.installed.SetSize(m.width, contentHeight)

	case Version:
		m.version.SetSize(m.width, contentHeight)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(utils.BackMsg); ok {
		m.screen = MainMenuScreen
		m.updateLayout()
		return m, nil
	}

	switch m.screen {
	case InstalledScreen:
		updatedModel, cmd := m.installed.Update(msg)
		m.installed = updatedModel.(utils.InstalledModel)

		return m, cmd
	case Version:
		if ws, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = ws.Width
			m.height = ws.Height
			m.updateLayout()
			return m, nil
		}

		updateModel, cmd := m.version.Update(msg)
		m.version = updateModel.(utils.VersionModel)
		return m, cmd

	case MainMenuScreen:
		return m.updateMainMenu(msg)
	}

	return m, nil
}

func (m model) updateMainMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd1 tea.Cmd
	var installedCmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "enter" {
			selected, ok := m.list.SelectedItem()
			if !ok {
				break
			}
			switch selected.TitleText {
			case "Install":

			case "Installed":
				m.installed = utils.NewInstalledModel()
				m.screen = InstalledScreen
				m.updateLayout()

				return m, m.installed.Init()
			case "Check For Update":

			case "Repair":

			case "Doctor":

			case "Completion":

			case "Version":
				m.screen = Version
				m.updateLayout()

				return m, m.version.Init()

			case "Exit":
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

		updatedInstalled, ic := m.installed.Update(msg)
		m.installed = updatedInstalled.(utils.InstalledModel)
		installedCmd = ic
	}

	m.loadingSpinner, cmd1 = m.loadingSpinner.Update(msg)
	cmd2 := m.list.Update(msg)
	cmd4 := m.packageManager.Update(msg)

	return m, tea.Batch(cmd1, cmd2, cmd4, installedCmd)
}

func (m model) footerText() string {
	switch m.screen {

	case MainMenuScreen:
		return "↑/k up • ↓/j down • enter select • q quit"

	case InstalledScreen:
		return "↑/k up • ↓/j down • esc back"

	case Version:
		return "esc • back"
	}

	return ""
}
func (m model) View() tea.View {
	var screenContent string

	footer := ui.NewFooter(
		m.width,
		m.footerText(),
	).View()

	switch m.screen {

	case InstalledScreen:
		screenContent = m.installed.View().Content

	case Version:
		screenContent = m.version.View().Content

	case MainMenuScreen:
		listView := m.list.View()

		availableHeight := m.height -
			lipgloss.Height(m.header()) -
			lipgloss.Height(m.description()) -
			lipgloss.Height(m.buildListTitle()) -
			lipgloss.Height(footer)

		if availableHeight < 1 {
			availableHeight = 1
		}

		listView = lipgloss.NewStyle().
			Height(availableHeight).
			Render(listView)

		screenContent = lipgloss.JoinVertical(
			lipgloss.Left,
			m.buildListTitle(),
			listView,
		)

	default:
		screenContent = "Unknown Screen"
	}

	top := lipgloss.JoinVertical(
		lipgloss.Left,
		m.header(),
		m.description(),
		screenContent,
	)

	// Calculate exactly how much space is left before the footer.
	spacerHeight := m.height - lipgloss.Height(top) - lipgloss.Height(footer)

	if spacerHeight < 0 {
		spacerHeight = 0
	}

	spacer := lipgloss.NewStyle().
		Height(spacerHeight).
		Render("")

	screen := lipgloss.JoinVertical(
		lipgloss.Left,
		top,
		spacer,
		footer,
	)

	view := tea.NewView(screen)
	view.AltScreen = true

	return view
}

func installPackage(pkg string) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return ui.InstalledPackageMsg{
			Package: pkg,
		}
	})
}

func main() {
	p := tea.NewProgram(newModel())
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}
}
