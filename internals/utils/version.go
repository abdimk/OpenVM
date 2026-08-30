package utils

import (
	tea "charm.land/bubbletea/v2"
	"github.com/abdimk/openvm/cmd/ui"
)

const version = "0.0.1"

type VersionModel struct {
	width  int
	height int

	pager ui.Pager
}

func NewVersionModel() VersionModel {
	pager := ui.NewPager(
		"OpenVM Version",
		"Your installed tools context here",
	)
	pager.HideFooter()
	return VersionModel{
		pager: pager,
	}
}

func (v VersionModel) Init() tea.Cmd {
	return v.pager.Init()
}

func (v VersionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return v, func() tea.Msg { return BackMsg{} }
		}
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
	}

	v.pager, cmd = v.pager.Update(msg)

	return v, cmd
}

// SetSize resizes the version view to the given content area so the
// shared footer (help text + separator) rendered by the parent layout
// stays visible at the bottom of the screen.
func (v *VersionModel) SetSize(width, height int) {
	v.width = width
	v.height = height

	v.pager, _ = v.pager.Update(tea.WindowSizeMsg{
		Width:  width,
		Height: height,
	})
}

func (v VersionModel) View() tea.View {
	return tea.NewView(v.pager.View())
}
