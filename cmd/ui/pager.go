package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	pagerTitleStyle = func() lipgloss.Style {
		border := lipgloss.RoundedBorder()
		border.Right = "├"

		return lipgloss.NewStyle().
			BorderStyle(border).
			Padding(0, 1)
	}()

	pagerInfoStyle = func() lipgloss.Style {
		border := lipgloss.RoundedBorder()
		border.Left = "┤"

		return pagerTitleStyle.BorderStyle(border)
	}()
)

type Pager struct {
	title      string
	content    string
	showFooter bool
	ready      bool
	viewport   viewport.Model
}

func NewPager(title, content string) Pager {
	return Pager{
		title:      title,
		content:    content,
		showFooter: true,
	}
}

// HideFooter disables the pager's own scroll-percentage footer so the view
// relies on the shared global footer instead.
func (p *Pager) HideFooter() {
	p.showFooter = false
}

func (p Pager) Init() tea.Cmd {
	return nil
}

func (p Pager) Update(msg tea.Msg) (Pager, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(p.headerView())

		var footerHeight int
		if p.showFooter {
			footerHeight = lipgloss.Height(p.footerView())
		}

		verticalMarginHeight := headerHeight + footerHeight

		if !p.ready {
			p.viewport = viewport.New(
				viewport.WithWidth(msg.Width),
				viewport.WithHeight(msg.Height-verticalMarginHeight),
			)

			p.viewport.YPosition = headerHeight
			p.viewport.SetContent(p.content)

			p.ready = true
		} else {
			p.viewport.SetWidth(msg.Width)
			p.viewport.SetHeight(msg.Height - verticalMarginHeight)
		}
	}

	if p.ready {
		p.viewport, cmd = p.viewport.Update(msg)
	}

	return p, cmd
}

func (p Pager) View() string {
	if !p.ready {
		return "\nInitializing..."
	}

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		p.headerView(),
		p.viewport.View(),
	)
	if p.showFooter {
		view = lipgloss.JoinVertical(
			lipgloss.Left,
			view,
			p.footerView(),
		)
	}
	return view
}

func (p *Pager) SetContent(content string) {
	p.content = content

	if p.ready {
		p.viewport.SetContent(content)
	}
}

func (p *Pager) SetTitle(title string) {
	p.title = title
}

func (p Pager) headerView() string {
	title := pagerTitleStyle.Render(p.title)

	line := strings.Repeat(
		"─",
		max(
			0,
			p.viewport.Width()-lipgloss.Width(title),
		),
	)

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		title,
		line,
	)
}

func (p Pager) footerView() string {
	info := pagerInfoStyle.Render(
		fmt.Sprintf(
			"%3.f%%",
			p.viewport.ScrollPercent()*100,
		),
	)

	line := strings.Repeat(
		"─",
		max(
			0,
			p.viewport.Width()-lipgloss.Width(info),
		),
	)

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		line,
		info,
	)
}
