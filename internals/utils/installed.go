package utils

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
)

type BackMsg struct{}


var checkMark = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")

type InstalledModel struct {
	width  int
	height int

	packageManager *ui.PackageManagerModel
	completed      []string
}

func NewInstalledModel() InstalledModel {
	packages := []string{
		"spicerack-4.7.8",
		"schnurrkit-4.3.6",
		"libtacos-1.2.7",
		"babys-monads-8.8.2",
		"hojicha-2.8.0",
		"bad-kitty-4.7.8",
		"molasses-utils-2.9.3",
		"eggy-7.8.1",
		"jalapeño-6.1.9",
		"libyuzu-8.6.7",
		"cashew-apple-2.0.0",
		"currykit-9.8.4",
		"xmodmeow-4.3.8",
		"currywurst-devel-4.1.2",
		"snow-peas-0.8.3",
		"coffee-CUPS-3.1.6",
		"fullenglish-2.6.0",
		"libgardening-2.2.8",
		"libesszet-0.3.0",
		"rock-lobster-9.5.6",
		"licorice-utils-4.1.2",
		"old-socks-devel-1.9.8",
		"libpurring-7.4.7",
		"zeichenorientierte-benutzerschnittstellen-2.5.5",
		"standmixer-3.6.0",
		"chai-1.2.0",
		"vegeutils-7.1.0",
		"xkohlrabi-9.7.5",
	}

	m := InstalledModel{
		packageManager: ui.NewPackageManager(
			packages,
		
			func(pkg string) tea.Cmd {
				return tea.Tick(time.Second, func(time.Time) tea.Msg {
					return ui.InstalledPackageMsg{Package: pkg}
				})
			},
		),
	}


	m.packageManager.SetSize(80, 20)
	return m
}

func (m InstalledModel) Init() tea.Cmd {
	if m.packageManager == nil {
		return nil
	}
	return m.packageManager.Init()
}

func (m *InstalledModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	if m.packageManager != nil {
		m.packageManager.SetSize(width, height)
	}
}
func (m InstalledModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.packageManager != nil {
			m.packageManager.SetSize(m.width, m.height)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg { return BackMsg{} }
		}

	case ui.InstalledPackageMsg:
	
		if m.packageManager != nil && !m.packageManager.Done() {
			if pkg := m.packageManager.CurrentPackage(); pkg != "" {
				m.completed = append(m.completed, pkg)
			}
		}
	}


	if m.packageManager != nil {
		if cmd := m.packageManager.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m InstalledModel) View() tea.View {
	var b strings.Builder

	for _, pkg := range m.completed {
		b.WriteString(checkMark.String() + " " + pkg + "\n")
	}

	if m.packageManager != nil {
		b.WriteString(m.packageManager.View())
	}

	content := b.String()


	if m.height > 0 {
		pad := m.height - lipgloss.Height(content)
		if pad > 0 {
			content += strings.Repeat("\n", pad)
		}
	}

	return tea.NewView(content)
}
