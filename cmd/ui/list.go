package ui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2" 
)

type Item struct {
	TitleText       string
	DescriptionText string
}

func (i Item) Title() string {
	return i.TitleText
}

func (i Item) Description() string {
	return i.DescriptionText
}

func (i Item) FilterValue() string {
	return i.TitleText
}

type ListModel struct {
	list list.Model
}

func New(
	title string,
	items []Item,
	width int,
	height int,
) *ListModel {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	// Create the default delegate and override the pink selected styles
	delegate := list.NewDefaultDelegate()

	// Change selected item color from pink to your custom color
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#743054")).
		BorderLeftForeground(lipgloss.Color("#743054"))

	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#743054")).
		BorderLeftForeground(lipgloss.Color("#743054"))

	// Optional: also customize unselected items
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(lipgloss.Color("#FFFDF5"))
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.
		Foreground(lipgloss.Color("#A49F96"))

	l := list.New(
		listItems,
		delegate, // pass your customized delegate
		width,
		height,
	)

	l.Title = title
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)

	return &ListModel{
		list: l,
	}
}

func (m *ListModel) Init() tea.Cmd {
	return nil
}

func (m *ListModel) Update(msg tea.Msg) tea.Cmd {
	newList, cmd := m.list.Update(msg)
	m.list = newList
	return cmd
}

func (m ListModel) View() string {
	return m.list.View()
}

func (m *ListModel) SetSize(width, height int) {
	m.list.SetSize(width, height)
}
func (m *ListModel) SetTitle(title string) {
	m.list.Title = title
}
func (m *ListModel) SetItems(items []Item) tea.Cmd {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}
	return m.list.SetItems(listItems)
}

func (m *ListModel) InsertItem(index int, item Item) tea.Cmd {
	return m.list.InsertItem(index, item)
}

func (m *ListModel) SelectedItem() (Item, bool) {
	item := m.list.SelectedItem()
	if item == nil {
		return Item{}, false
	}
	selected, ok := item.(Item)
	return selected, ok
}