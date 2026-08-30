package utils

import tea "charm.land/bubbletea/v2"

type UpdateCheckModel struct {
	info InfoModel
}

func NewUpdateCheckModel() UpdateCheckModel {
	return UpdateCheckModel{
		info: NewInfoModel(
			"Check For Update",
			"Check for and update installed tools.",
		),
	}
}

func (m UpdateCheckModel) Init() tea.Cmd {
	return m.info.Init()
}

func (m UpdateCheckModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	inner, cmd := m.info.Update(msg)
	m.info = inner.(InfoModel)
	return m, cmd
}

func (m *UpdateCheckModel) SetSize(width, height int) {
	m.info.SetSize(width, height)
}

func (m UpdateCheckModel) View() tea.View {
	return m.info.View()
}
