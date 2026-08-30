package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)


type InstalledPackageMsg struct {
	Package string
}


type PackageManagerModel struct {
	packages []string
	index    int
	width    int
	height   int

	spinner  spinner.Model
	progress progress.Model

	done bool

	install func(string) tea.Cmd
}

var (
	currentPackageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF00"))

	doneStyle = lipgloss.NewStyle().
			Margin(1, 2)

	checkMarkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))
)


func NewPackageManager(
	packages []string,
	install func(string) tea.Cmd,
) *PackageManagerModel {

	p := progress.New(
		progress.WithDefaultBlend(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	s := spinner.New()
	s.Style = lipgloss.NewStyle().
		Foreground(lipgloss.Color("63"))

	return &PackageManagerModel{
		packages: packages,
		index:    0,
		spinner:  s,
		progress: p,
		install:  install,
	}
}


func (m *PackageManagerModel) Init() tea.Cmd {
	if len(m.packages) == 0 {
		m.done = true
		return nil
	}

	return tea.Batch(
		m.spinner.Tick,
		m.install(m.packages[m.index]),
	)
}

func (m *PackageManagerModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case InstalledPackageMsg:
		
		if m.done {
			return nil
		}

	
		currentPackage := m.packages[m.index]

	
		if m.index >= len(m.packages)-1 {
			m.done = true

			return m.progress.SetPercent(1)
		}

		m.index++

		progressCmd := m.progress.SetPercent(
			float64(m.index) / float64(len(m.packages)),
		)

	
		installCmd := m.install(m.packages[m.index])

	
		_ = currentPackage

		return tea.Batch(
			progressCmd,
			installCmd,
		)

	case spinner.TickMsg:
		if m.done {
			return nil
		}

		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return cmd

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return cmd
	}

	return nil
}


func (m PackageManagerModel) View() string {
	if len(m.packages) == 0 {
		return "No packages to install."
	}

	if m.done {
		return doneStyle.Render(
			checkMarkStyle.Render("✓") +
				fmt.Sprintf(" Done! Installed %d packages.", len(m.packages)),
		)
	}

	total := len(m.packages)

	countWidth := lipgloss.Width(fmt.Sprintf("%d", total))


	packageCount := fmt.Sprintf(
		" %*d/%*d",
		countWidth,
		m.index+1,
		countWidth,
		total,
	)

	spin := m.spinner.View() + " "
	prog := m.progress.View()


	cellsAvailable := max(
		0,
		m.width-lipgloss.Width(
			spin+prog+packageCount,
		),
	)

	packageName := currentPackageStyle.Render(
		m.packages[m.index],
	)

	info := lipgloss.NewStyle().
		MaxWidth(cellsAvailable).
		Render("Installing " + packageName)

	// Fill the remaining space between info and progress.
	cellsRemaining := max(
		0,
		m.width-lipgloss.Width(
			spin+info+prog+packageCount,
		),
	)

	gap := strings.Repeat(" ", cellsRemaining)

	return spin + info + gap + prog + packageCount
}

// SetSize updates the available component size.
func (m *PackageManagerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Done returns true when every package has been installed.
func (m *PackageManagerModel) Done() bool {
	return m.done
}

// CurrentPackage returns the package currently being installed.
func (m *PackageManagerModel) CurrentPackage() string {
	if len(m.packages) == 0 || m.done {
		return ""
	}

	return m.packages[m.index]
}

// TotalPackages returns the total number of packages.
func (m *PackageManagerModel) TotalPackages() int {
	return len(m.packages)
}

// CurrentIndex returns the current package index.
func (m *PackageManagerModel) CurrentIndex() int {
	return m.index
}