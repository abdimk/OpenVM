package utils

import tea "charm.land/bubbletea/v2"

type RepairModel struct {
	info InfoModel
}

func NewRepairModel() RepairModel {
	return RepairModel{
		info: NewInfoModel(
			"Repair",
			"Detect and repair broken tool installations.",
		),
	}
}

func (m RepairModel) Init() tea.Cmd {
	return m.info.Init()
}

func (m RepairModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	inner, cmd := m.info.Update(msg)
	m.info = inner.(InfoModel)
	return m, cmd
}

func (m *RepairModel) SetSize(width, height int) {
	m.info.SetSize(width, height)
}

func (m RepairModel) View() tea.View {
	return m.info.View()
}
