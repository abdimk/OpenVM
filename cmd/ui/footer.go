package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type Footer struct {
	Width int
	Text  string
}

func NewFooter(width int, text string) Footer {
	return Footer{
		Width: width,
		Text:  text,
	}
}

func (f Footer) View() string {
	line := strings.Repeat("─", f.Width)

	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Padding(0, 2)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		line,
		textStyle.Render(f.Text),
	)
}