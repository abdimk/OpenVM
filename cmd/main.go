package main

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
	// "github.com/abdimk/openvm/internal/utils"
)

type model struct {
	width          int
	height         int
	loadingSpinner ui.Spinner
	list           *ui.ListModel
	table          *ui.TableModel
	packageManager *ui.PackageManagerModel
}

func newModel() *model {
	items := []ui.Item{
		{TitleText: "Installed", DescriptionText: "List all the installed languages and tools"},
		{TitleText: "Completion", DescriptionText: "Generate shell Completion scripts"},
		{TitleText: "Doctor", DescriptionText: "Check enviroment and print connection info"},
		{TitleText: "Mock", DescriptionText: "Mock data generation commands"},
		{TitleText: "Receiver", DescriptionText: "Print version information"},
		{TitleText: "Whoami", DescriptionText: "Show the current user"},
		{TitleText: "Version", DescriptionText: "Check the version information for OpenVM"},
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

func (m *model) updateLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	headerHeight := lipgloss.Height(m.header())
	descriptionHeight := lipgloss.Height(m.description())
	spinnerHeight := lipgloss.Height(m.loadingSpinner.View())

	usedHeight := headerHeight + descriptionHeight + spinnerHeight
	listHeight := m.height - usedHeight
	if listHeight < 1 {
		listHeight = 1
	}

	m.list.SetSize(m.width, listHeight)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd1 tea.Cmd

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
			case "Version":
				fmt.Println("Selected:", selected.TitleText)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
	}

	m.loadingSpinner, cmd1 = m.loadingSpinner.Update(msg)
	cmd2 := m.list.Update(msg)
	cmd4 := m.packageManager.Update(msg)

	return m, tea.Batch(cmd1, cmd2, cmd4)
}

func (m model) View() tea.View {
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.header(),
		m.description(),
		m.list.View(),
	)
	view := tea.NewView(content)
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