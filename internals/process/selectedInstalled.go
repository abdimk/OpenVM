package process

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
	"github.com/abdimk/openvm/internals/api"
	"github.com/abdimk/openvm/internals/utils"
)

// GoVersionsFetchedMsg is sent once the Go release list has been downloaded
// (successfully or not) from go.dev.
type GoVersionsFetchedMsg struct {
	Releases []api.Release
	Err      error
}

// versionEntry couples a list item with the release/file data behind it.
type versionEntry struct {
	title   string
	release api.Release
	file    api.File
	current bool
}

type SelectedInstalledModel struct {
	language utils.Language
	width    int
	height   int

	versions *ui.ListModel
	entries  []versionEntry

	loading bool
	err     error

	spinner ui.Spinner
}

func NewSelectedInstalledModel(lang utils.Language) SelectedInstalledModel {
	m := SelectedInstalledModel{
		language: lang,
		loading:  isGo(lang),
		spinner:  ui.SpinnerModel("Fetching available versions..."),
	}
	m.versions = ui.New("", nil, 80, 20)
	return m
}

func isGo(lang utils.Language) bool {
	return strings.EqualFold(lang.Name, utils.Go)
}

func (m SelectedInstalledModel) Init() tea.Cmd {
	if !isGo(m.language) {
		return nil
	}

	return tea.Batch(
		m.spinner.Init(),
		fetchGoVersionsCmd(),
	)
}

func fetchGoVersionsCmd() tea.Cmd {
	return func() tea.Msg {
		releases, err := api.ReleasesForMachine()
		return GoVersionsFetchedMsg{Releases: releases, Err: err}
	}
}

func (m *SelectedInstalledModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// View() renders: header line, blank line, status/title line, then the
	// list, a blank line, and the detail pane — so the list gets what's left
	// after those 4 fixed lines plus the detail pane's height.
	chrome := 4

	detail := m.detailView()
	chrome += lipgloss.Height(detail)

	listHeight := height - chrome
	if listHeight < 1 {
		listHeight = 1
	}

	if m.versions != nil {
		m.versions.SetSize(width, listHeight)
	}
}

func (m SelectedInstalledModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg { return utils.BackMsg{} }
		}

	case GoVersionsFetchedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			m.SetSize(m.width, m.height)
			return m, nil
		}

		m.err = nil
		m.entries = buildVersionEntries(m.language, msg.Releases)

		items := make([]ui.Item, len(m.entries))
		for i, e := range m.entries {
			items[i] = ui.Item{
				TitleText:       e.title,
				DescriptionText: entryDescription(e),
			}
		}
		if m.versions != nil {
			_ = m.versions.SetItems(items)
		}

		// The detail pane grows once entries exist, so re-budget the list.
		m.SetSize(m.width, m.height)
		return m, nil
	}

	var cmds []tea.Cmd

	if m.loading {
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	if m.versions != nil {
		cmds = append(cmds, m.versions.Update(msg))
	}

	return m, tea.Batch(cmds...)
}

// buildVersionEntries turns the fetched releases into list entries filtered
// to the current machine, with the currently installed version first.
func buildVersionEntries(lang utils.Language, releases []api.Release) []versionEntry {
	current := extractGoVersion(lang.Version)

	entries := make([]versionEntry, 0, len(releases)+1)
	currentIdx := -1

	for _, r := range releases {
		file, ok := r.FileForMachine()
		if !ok {
			continue // no download for this OS/arch
		}

		e := versionEntry{
			title:   r.Version,
			release: r,
			file:    file,
			current: current != "" && r.Version == current,
		}
		if e.current {
			currentIdx = len(entries)
		}
		entries = append(entries, e)
	}

	// The installed version is not in the archive (e.g. a custom/devel
	// build): still show it as the first entry, without download info.
	if current != "" && currentIdx < 0 {
		entries = append([]versionEntry{{
			title:   current,
			release: api.Release{Version: current},
			current: true,
		}}, entries...)
		currentIdx = 0
	}

	// Move the current entry to the top of the list.
	if currentIdx > 0 {
		e := entries[currentIdx]
		copy(entries[1:currentIdx+1], entries[0:currentIdx])
		entries[0] = e
	}

	return entries
}

// extractGoVersion pulls the "goX.Y.Z" token out of a `go version` output
// such as "go version go1.25.2 windows/amd64".
func extractGoVersion(versionOutput string) string {
	for _, field := range strings.Fields(versionOutput) {
		if strings.HasPrefix(field, "go1.") {
			return field
		}
	}
	return ""
}

func entryDescription(e versionEntry) string {
	if e.file.Filename == "" {
		return "Currently installed on this machine"
	}

	stability := "archived"
	if e.release.Stable {
		stability = "stable"
	}

	return fmt.Sprintf("%s • %s/%s • %s • %.2f MB",
		stability, e.file.OS, e.file.Arch, e.file.Filename, e.file.SizeMB())
}

func (m SelectedInstalledModel) selectedEntry() (versionEntry, bool) {
	if m.versions == nil {
		return versionEntry{}, false
	}

	item, ok := m.versions.SelectedItem()
	if !ok {
		return versionEntry{}, false
	}

	for _, e := range m.entries {
		if e.title == item.TitleText {
			return e, true
		}
	}
	return versionEntry{}, false
}

func (m SelectedInstalledModel) detailView() string {
	entry, ok := m.selectedEntry()
	if !ok {
		return ""
	}

	sep := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#444444")).
		Render(strings.Repeat("─", max(40, m.width-4)))

	label := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	value := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff"))

	var b strings.Builder
	b.WriteString(sep)
	b.WriteString("\n")

	title := entry.title
	if entry.current {
		title += " (current)"
	}
	b.WriteString(label.Render(" Version:  ") + value.Render(title) + "\n")

	if entry.file.Filename == "" {
		b.WriteString(label.Render(" Path:     ") + value.Render(m.language.Path) + "\n")
		b.WriteString(label.Render(" Status:   ") +
			value.Render(fmt.Sprintf(
				"No matching download found in the Go archive for %s/%s",
				api.MachineOS(), api.MachineArch())) + "\n")
		return b.String()
	}

	stability := "archived"
	if entry.release.Stable {
		stability = "stable release"
	}

	url := "https://go.dev/dl/" + entry.file.Filename
	link := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#58A6FF")).
		Hyperlink(url).
		Render(url)

	sha := entry.file.SHA256
	if len(sha) > 12 {
		sha = sha[:12] + "…"
	}

	b.WriteString(label.Render(" Status:   ") + value.Render(stability) + "\n")
	b.WriteString(label.Render(" File:     ") + value.Render(entry.file.Filename) + "\n")
	b.WriteString(label.Render(" Target:   ") +
		value.Render(fmt.Sprintf("%s/%s", entry.file.OS, entry.file.Arch)) + "\n")
	b.WriteString(label.Render(" Size:     ") +
		value.Render(fmt.Sprintf("%.2f MB", entry.file.SizeMB())) + "\n")
	b.WriteString(label.Render(" SHA256:   ") + value.Render(sha) + "\n")
	b.WriteString(label.Render(" URL:      ") + link)

	return b.String()
}

func (m SelectedInstalledModel) buildHeader() string {
	if m.width <= 0 {
		return fmt.Sprintf("Language: [%s]", m.language.Name)
	}

	leftBlock := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		PaddingLeft(2).
		PaddingBottom(0).
		Render("Language:")

	greenStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ff00")).
		Render(fmt.Sprintf(" [%s]", m.language.Name))

	rightBlock := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		PaddingRight(2).
		PaddingLeft(1).
		Render(fmt.Sprintf("Machine: [%s]", utils.GetMachineType()))

	leftW := lipgloss.Width(leftBlock) + lipgloss.Width(greenStyle)
	rightW := lipgloss.Width(rightBlock)

	gap := m.width - leftW - rightW
	if gap < 0 {
		gap = 0
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftBlock,
		greenStyle,
		strings.Repeat(" ", gap),
		rightBlock,
	)
}

func (m SelectedInstalledModel) View() tea.View {
	var content strings.Builder

	content.WriteString(m.buildHeader())
	content.WriteString("\n\n")

	switch {
	case m.loading:
		content.WriteString(m.spinner.View())
		content.WriteString("\n")

	case m.err != nil:
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff5555")).
			Render(fmt.Sprintf("Failed to fetch available versions: %v", m.err)))
		content.WriteString("\n")

	case len(m.entries) > 0:
		title := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			PaddingLeft(2).
			Render(fmt.Sprintf("Available Go Versions (%d)", len(m.entries)))

		content.WriteString(title)
		content.WriteString("\n")

		if m.versions != nil {
			content.WriteString(m.versions.View())
		}
		content.WriteString("\n")

	default:
		// Non-Go languages (or Go with no data yet): plain info card.
		path := m.language.Path
		version := m.language.Version
		status := "Installed"
		if path == "" {
			path = "Not found on this machine"
			version = "—"
			status = "Not installed"
		}

		body := lipgloss.NewStyle().PaddingLeft(2).Render(
			fmt.Sprintf("Name: %s\nPath: %s\nVersion: %s\nStatus: %s",
				m.language.Name, path, version, status),
		)
		content.WriteString(body)
	}

	detail := m.detailView()
	if detail != "" {
		content.WriteString("\n")
		content.WriteString(detail)
	}

	screen := content.String()

	if m.height > 0 {
		pad := m.height - lipgloss.Height(screen)
		if pad > 0 {
			screen += strings.Repeat("\n", pad)
		}
	}

	return tea.NewView(screen)
}
