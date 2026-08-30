package utils

import tea "charm.land/bubbletea/v2"

type InstallModel struct {
	info InfoModel
}

func NewInstallModel() InstallModel {
	return InstallModel{
		info: NewInfoModel(
			"Install",
			"Choose a programming language or development tool to install.",
		),
	}
}

func (m InstallModel) Init() tea.Cmd {
	return m.info.Init()
}

func (m InstallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	inner, cmd := m.info.Update(msg)
	m.info = inner.(InfoModel)
	return m, cmd
}

func (m *InstallModel) SetSize(width, height int) {
	m.info.SetSize(width, height)
}

func (m InstallModel) View() tea.View {
	return m.info.View()
}
