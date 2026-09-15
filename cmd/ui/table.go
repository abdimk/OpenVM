package ui

import (
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type TableModel struct {
	table table.Model
}

func NewTable(
	columns []table.Column,
	rows []table.Row,
	width int,
	height int,
) *TableModel {

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithWidth(width),
		table.WithHeight(height),
	)

	styles := table.DefaultStyles()

	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)

	styles.Selected = styles.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.SetStyles(styles)

	return &TableModel{
		table: t,
	}
}

func (m *TableModel) Init() tea.Cmd {
	return nil
}

func (m *TableModel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	m.table, cmd = m.table.Update(msg)

	return cmd
}

func (m TableModel) View() string {
	return m.table.View()
}

func (m *TableModel) SetSize(width, height int) {
	m.table.SetWidth(width)
	m.table.SetHeight(height)
}

func (m *TableModel) SetRows(rows []table.Row) {
	m.table.SetRows(rows)
}

func (m *TableModel) SelectedRow() table.Row {
	return m.table.SelectedRow()
}
