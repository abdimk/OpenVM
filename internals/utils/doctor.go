package utils

import tea "charm.land/bubbletea/v2"

type DoctorModel struct {
	info InfoModel
}

func NewDoctorModel() DoctorModel {
	return DoctorModel{
		info: NewInfoModel(
			"Doctor",
			"Check your environment and diagnose configuration issues.",
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

func (d DoctorModel) View() tea.View {
	return d.info.View()
}
