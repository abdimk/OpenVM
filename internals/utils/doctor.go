package utils

import tea "charm.land/bubbletea/v2"

type DoctorModel struct {
	info InfoModel
}

func NewDockerModel() DoctorModel {
	return DoctorModel{
		info: NewInfoModel(
			"Doctor",
			"Manage Docker containers and images.",
		),
	}
}

func (d DoctorModel) Init() tea.Cmd {
	return d.info.Init()
}

func (d DoctorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	inner, cmd := d.info.Update(msg)
	d.info = inner.(InfoModel)
	return d, cmd
}

func (d *DoctorModel) SetSize(width, height int) {
	d.info.SetSize(width, height)
}

func (m DoctorModel) View() tea.View {
	return m.info.View()
}
