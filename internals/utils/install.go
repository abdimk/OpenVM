package utils

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/abdimk/openvm/internals/system"
)

type InstallModel struct {
	info InfoModel
}

func NewInstallModel() InstallModel {
	result := system.GetLanguages()
	return InstallModel{
		info: NewInfoModel(
			"Install",
			fmt.Sprintf("Choose a programming language or development tool to install.\n %s", result),
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
