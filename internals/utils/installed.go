package utils

import tea "charm.land/bubbletea/v2"

type BackMsg struct{}

type InstalledModel struct {
}

func (m InstalledModel) Init() tea.Cmd {
	return nil
}

func (m InstalledModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "backspace":
			return m, func() tea.Msg {
				return BackMsg{}
			}
		}
	}

	return m, nil
}

func (m InstalledModel) View() tea.View {
	return tea.NewView(
		"Installed Section\n\n" +
			"Press ESC to go back",
	)
}