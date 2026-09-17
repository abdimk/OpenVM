package ui

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Spinner struct {
	model spinner.Model
	text  string
}

func SpinnerModel(text string) Spinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().
		Foreground(lipgloss.Color("69"))

	return Spinner{
		model: s,
		text:  text,
	}
}

func (s Spinner) Init() tea.Cmd {
	return s.model.Tick
}

func (s Spinner) WithText(text string) Spinner {
	s.text = text
	return s
}

func (s Spinner) Update(msg tea.Msg) (Spinner, tea.Cmd) {
	var cmd tea.Cmd
	s.model, cmd = s.model.Update(msg)
	return s, cmd
}

func (s Spinner) View() string {
	return s.model.View() + " " + s.text
}
