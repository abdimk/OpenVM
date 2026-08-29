package utils

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/abdimk/openvm/cmd/ui"
)

type BackMsg struct{}

type InstalledModel struct {
	width int
	height int
	packageManager *ui.PackageManagerModel
	
}


func NewInstalledModel() *InstalledModel{
	packages := []string{
		"go",
		"node",
		"python",
		"rust",
	}
	m := InstalledModel{
			packageManager: ui.NewPackageManager(
				packages,
				// Dummy install: just waits 1 second then reports "done"
				func(pkg string) tea.Cmd {
					return tea.Tick(time.Second, func(time.Time) tea.Msg {
						return ui.InstalledPackageMsg{
							Package: pkg,
						}
					})
				},
			),
		}
	
		m.packageManager.SetSize(80, 20)
		
		return &m
}

func (m InstalledModel) Init() tea.Cmd {
	if m.packageManager == nil{
		return nil
	}
	return m.packageManager.Init()
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
			return m, func() tea.Msg {
				return BackMsg{}
			}
		}
	}

	if m.packageManager != nil {
		cmd := m.packageManager.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m InstalledModel) View() tea.View {
	var content string

	if m.packageManager != nil {
		content = m.packageManager.View()
	} else {
		content = "Installed Section"
	}

	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Render("\n\nPress ESC or Backspace to go back")

	v := tea.NewView(content + hint)
	return v
}