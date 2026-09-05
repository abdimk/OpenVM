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
	if f.Width <= 0 {
		return ""
	}

	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Padding(0, 2)


	lineStyle := lipgloss.NewStyle().
		MarginBottom(1).
		Render(strings.Repeat("─", f.Width))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		textStyle.Render(f.Text),
		lineStyle,
	)
}
