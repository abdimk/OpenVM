package ui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	progressPadding = 2
	progressMaxWidth = 80
)

var progressHelpStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#626262"))

type progressTickMsg time.Time


type Progress struct {
	model progress.Model
}


func NewProgress() Progress {
	return Progress{
		model: progress.New(progress.WithDefaultBlend()),
	}
}


func (p Progress) Init() tea.Cmd {
	return progressTickCmd()
}


func (p Progress) Update(msg tea.Msg) (Progress, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		width := msg.Width - progressPadding*2 - 4

		if width > progressMaxWidth {
			width = progressMaxWidth
		}

		if width < 0 {
			width = 0
		}

		p.model.SetWidth(width)

		return p, nil

	case progressTickMsg:
		if p.model.Percent() >= 1.0 {
			return p, nil
		}

		cmd := p.model.IncrPercent(0.25)

		return p, tea.Batch(
			progressTickCmd(),
			cmd,
		)

	case progress.FrameMsg:
		var cmd tea.Cmd

		p.model, cmd = p.model.Update(msg)

		return p, cmd
	}

	return p, nil
}


func (p Progress) View() string {
	pad := strings.Repeat(" ", progressPadding)

	return "\n" +
		pad + p.model.View() + "\n\n" +
		pad + progressHelpStyle.Render("Press any key to quit")
}


func (p Progress) Percent() float64 {
	return p.model.Percent()
}

// Bar renders just the animated progress bar itself, without the help text
// or surrounding padding — suitable for embedding in a header line.
func (p Progress) Bar() string {
	return p.model.View()
}

// SetBarWidth resizes the bar (percentage of screen width is applied by the
// caller). Pointer receiver: bubbles' SetWidth mutates the model, so a value
// receiver would silently discard the change on a copy.
func (p *Progress) SetBarWidth(width int) {
	p.model.SetWidth(width)
}

// SetPercent sets the bar to a given 0..1 value and returns the animation
// command that eases it there. Pointer receiver for the same reason.
func (p *Progress) SetPercent(v float64) tea.Cmd {
	return p.model.SetPercent(v)
}

func (p *Progress) IncrPercent(amount float64) tea.Cmd {
	return p.model.IncrPercent(amount)
}

func progressTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return progressTickMsg(t)
	})
}