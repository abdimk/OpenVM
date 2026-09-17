package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
	"github.com/abdimk/openvm/internals/process"
	"github.com/abdimk/openvm/internals/utils"
)

type Screen int

const (
	MainMenuScreen Screen = iota
	InstallScreen
	InstalledScreen
	UpdateCheckScreen
	RepairScreen
	DoctorScreen
	CurrentVersionScreen
	Version
	SelectedInstalledScreen
)

type model struct {
	width          int
	height         int
	loadingSpinner ui.Spinner
	list           *ui.ListModel

	screen            Screen
	selectedFrom      Screen
	installed         utils.InstalledModel
	version           utils.VersionModel
	install           utils.InstallModel
	update            utils.UpdateCheckModel
	repair            utils.RepairModel
	doctor            utils.DoctorModel
	current           utils.CurrentVersionModel
	selectedInstalled process.SelectedInstalledModel
}

func newModel() *model {
	items := []ui.Item{
		{TitleText: "Install", DescriptionText: "Install a programming language or development tool"},
		{TitleText: "Installed", DescriptionText: "List all installed languages and tools"},
		{TitleText: "Check For Update", DescriptionText: "Check for and update installed tools"},
		{TitleText: "Repair", DescriptionText: "Detect and repair broken tool installations"},
		{TitleText: "Doctor", DescriptionText: "Check your environment and diagnose configuration issues"},
		{TitleText: "Current Version", DescriptionText: "Current Version of Software you have"},
		{TitleText: "Version", DescriptionText: "Show OpenVM version information"},
		{TitleText: "Exit", DescriptionText: "Exit OpenVM"},
	}

	return &model{
		loadingSpinner: ui.SpinnerModel("Loading..."),
		list:           ui.New("Available Commands", items, 80, 20),
		screen:         MainMenuScreen,
		installed:      utils.NewInstalledModel(),
		version:        utils.NewVersionModel(),
		install:        utils.NewInstallModel(),
		update:         utils.NewUpdateCheckModel(),
		repair:         utils.NewRepairModel(),
		doctor:         utils.NewDoctorModel(),
		current:        utils.NewCurrentVersionModel(),
	}
}

func (m model) Init() tea.Cmd {
	return m.loadingSpinner.Init()
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
	if m.width <= 0 {
		return ""
	}

	description := "OpenVM is a Terminal User Interface (TUI) for managing development tools and programming languages.\n" +
		"It simplifies installing, updating, switching between versions, and repairing tools from one unified terminal interface."

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Width(m.width).
		Align(lipgloss.Center).
		MarginBottom(1)

	return style.Render(description)
}

func (m model) buildListTitle() string {
	if m.width <= 0 {
		return "Available Commands"
	}

	leftBlock := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		PaddingLeft(2).
		PaddingBottom(0).
		Render("Available Commands")

	rightBlock := lipgloss.NewStyle().
		PaddingRight(2).
		PaddingLeft(1).
		Render(utils.MachineLabel())
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

	if m.screen == Version {
		descriptionHeight = 0
	}

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

	case InstallScreen:
		m.install.SetSize(m.width, contentHeight)

	case UpdateCheckScreen:
		m.update.SetSize(m.width, contentHeight)

	case RepairScreen:
		m.repair.SetSize(m.width, contentHeight)

	case DoctorScreen:
		m.doctor.SetSize(m.width, contentHeight)

	case CurrentVersionScreen:
		m.current.SetSize(m.width, contentHeight)

	case Version:
		m.version.SetSize(m.width, contentHeight)

	case SelectedInstalledScreen:
		m.selectedInstalled.SetSize(m.width, contentHeight)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.height = ws.Height
		m.updateLayout()
		return m, nil
	}

	if _, ok := msg.(utils.BackMsg); ok {
		if m.screen == SelectedInstalledScreen {

			m.selectedInstalled.Cancel()
			utils.ClearFooterHint()

			back := m.selectedFrom
			if back == MainMenuScreen || back == SelectedInstalledScreen {
				back = InstalledScreen
			}
			m.screen = back
			m.selectedFrom = MainMenuScreen
		} else {
			m.screen = MainMenuScreen
		}
		m.updateLayout()

		if m.screen == InstalledScreen {
			return m, m.installed.Refresh()
		}
		return m, nil
	}

	if lsm, ok := msg.(utils.LanguageSelectedMsg); ok {
		m.selectedInstalled = process.NewSelectedInstalledModel(lsm.Language)
		m.selectedFrom = m.screen
		m.screen = SelectedInstalledScreen
		m.updateLayout()
		return m, m.selectedInstalled.Init()
	}

	switch m.screen {
	case InstalledScreen:
		updatedModel, cmd := m.installed.Update(msg)
		m.installed = updatedModel.(utils.InstalledModel)

		return m, cmd

	case SelectedInstalledScreen:
		updatedModel, cmd := m.selectedInstalled.Update(msg)
		m.selectedInstalled = updatedModel.(process.SelectedInstalledModel)
		return m, cmd

	case InstallScreen:
		um, cmd := m.install.Update(msg)
		m.install = um.(utils.InstallModel)
		return m, m.handleResize(msg, cmd)

	case UpdateCheckScreen:
		um, cmd := m.update.Update(msg)
		m.update = um.(utils.UpdateCheckModel)
		return m, m.handleResize(msg, cmd)

	case RepairScreen:
		um, cmd := m.repair.Update(msg)
		m.repair = um.(utils.RepairModel)
		return m, m.handleResize(msg, cmd)

	case DoctorScreen:
		um, cmd := m.doctor.Update(msg)
		m.doctor = um.(utils.DoctorModel)
		return m, m.handleResize(msg, cmd)

	case CurrentVersionScreen:
		um, cmd := m.current.Update(msg)
		m.current = um.(utils.CurrentVersionModel)
		return m, m.handleResize(msg, cmd)

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

func (m *model) handleResize(msg tea.Msg, cmd tea.Cmd) tea.Cmd {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.height = ws.Height
		m.updateLayout()
		return nil
	}
	return cmd
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
				m.screen = InstallScreen
				m.updateLayout()
				return m, m.install.Init()

			case "Installed":
				m.installed = utils.NewInstalledModel()
				utils.ClearFooterHint()
				m.screen = InstalledScreen
				m.updateLayout()

				return m, m.installed.Init()

			case "Check For Update":
				m.screen = UpdateCheckScreen
				m.updateLayout()
				return m, m.update.Init()

			case "Repair":
				m.screen = RepairScreen
				m.updateLayout()
				return m, m.repair.Init()

			case "Doctor":
				m.screen = DoctorScreen
				m.updateLayout()
				return m, m.doctor.Init()

			case "Current Version":
				m.screen = CurrentVersionScreen
				m.updateLayout()
				return m, m.current.Init()

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

	return m, tea.Batch(cmd1, cmd2, installedCmd)
}

func (m model) footerText() string {
	switch m.screen {

	case MainMenuScreen:
		return "↑/k up • ↓/j down • enter select • q quit"

	case InstallScreen:
		return "↑/k up • ↓/j down • esc back"

	case UpdateCheckScreen:
		return "esc • back"

	case RepairScreen:
		return "esc • back"

	case DoctorScreen:
		return "esc • back"

	case CurrentVersionScreen:
		return "esc • back"

	case InstalledScreen:
		if m.installed.Confirming() {
			return m.installed.ConfirmationView()
		}
		if hint := utils.CurrentFooterHint(); hint.Text != "" {
			st := lipgloss.NewStyle().Bold(true)
			if hint.Color != "" {
				st = st.Foreground(lipgloss.Color(hint.Color))
			}
			if hint.Bg != "" {
				st = st.Background(lipgloss.Color(hint.Bg))
			}
			return st.Render(hint.Text)
		}
		return "↑/k up • ↓/j down • enter select • u uninstall • esc back"

	case Version:
		return "esc • back"

	case SelectedInstalledScreen:
		if hint := utils.CurrentFooterHint(); hint.Text != "" {
			if hint.Color != "" {
				return lipgloss.NewStyle().
					Foreground(lipgloss.Color(hint.Color)).
					Bold(true).
					Render(hint.Text)
			}
			return hint.Text
		}
		if m.selectedInstalled.Busy() {
			return "downloading • esc cancel"
		}
		return "↑/k up • ↓/j down • enter install • esc back"
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

	case InstallScreen:
		screenContent = m.install.View().Content

	case UpdateCheckScreen:
		screenContent = m.update.View().Content

	case RepairScreen:
		screenContent = m.repair.View().Content

	case DoctorScreen:
		screenContent = m.doctor.View().Content

	case CurrentVersionScreen:
		screenContent = m.current.View().Content

	case Version:
		screenContent = m.version.View().Content

	case SelectedInstalledScreen:
		screenContent = m.selectedInstalled.View().Content

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

	topBlock := lipgloss.JoinVertical(
		lipgloss.Left,
		m.header(),
	)
	if m.screen != Version {
		topBlock = lipgloss.JoinVertical(
			lipgloss.Left,
			m.header(),
			m.description(),
		)
	}

	top := lipgloss.JoinVertical(
		lipgloss.Left,
		topBlock,
		screenContent,
	)

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

func main() {
	p := tea.NewProgram(newModel())
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}

}
