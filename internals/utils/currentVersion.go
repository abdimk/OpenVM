package utils

import tea "charm.land/bubbletea/v2"

type CurrentVersionModel struct {
	info InfoModel
}

func NewCurrentVersionModel() CurrentVersionModel {
	return CurrentVersionModel{
		info: NewInfoModel(
			"Current Version",
			"Current version of the software you have installed.",
		),
	}
}

func (m CurrentVersionModel) Init() tea.Cmd {
	return m.info.Init()
}

func (m CurrentVersionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	inner, cmd := m.info.Update(msg)
	m.info = inner.(InfoModel)
	return m, cmd
}

func (m *CurrentVersionModel) SetSize(width, height int) {
	m.info.SetSize(width, height)
}

func (m CurrentVersionModel) View() tea.View {
	return m.info.View()
}
