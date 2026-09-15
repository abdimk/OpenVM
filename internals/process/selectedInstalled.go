package process

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
	"github.com/abdimk/openvm/internals/api"
	"github.com/abdimk/openvm/internals/utils"
)

// progressDrainDoneMsg is a sentinel telling Update to stop re-arming the
// progress-event listener (the download goroutine has finished).
type progressDrainDoneMsg struct{}

// footerHintResetMsg clears a transient footer hint (e.g. the "you already
// have this version" notice) after its timeout elapses.
type footerHintResetMsg struct{}

// VersionsFetchedMsg is sent once the available version list has been
// downloaded (successfully or not) from the language's source (go.dev for Go,
// GitHub releases for Clang/LLVM).
type VersionsFetchedMsg struct {
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
	releases []api.Release

	loading bool
	err     error

	spinner ui.Spinner
	progress ui.Progress

	// Download/install pipeline state.
	phase          utils.DownloadPhase
	activeVersion  string
	lastPercent    float64
	downloaded     int64
	total          int64
	cancel         context.CancelFunc
	progressCh     <-chan utils.DownloadProgressMsg
	progressDone   <-chan struct{}
	seq            int    // pipeline instance tag; stale events are ignored
	currentVersion string // authoritative after a successful install
}

func NewSelectedInstalledModel(lang utils.Language) SelectedInstalledModel {
	m := SelectedInstalledModel{
		language: lang,
		loading:  isGo(lang) || isClang(lang),
		spinner:  ui.SpinnerModel("Fetching available versions..."),
		progress: ui.NewProgress(),
		phase:    utils.PhaseIdle,
	}
	m.versions = ui.New("", nil, 80, 20)
	return m
}

func isGo(lang utils.Language) bool {
	return strings.EqualFold(lang.Name, utils.Go)
}

// isClang reports whether the language maps to the C/C++ toolchain installs,
// which are managed from the LLVM (Clang) release packages.
func isClang(lang utils.Language) bool {
	name := strings.ToLower(strings.TrimSpace(lang.Name))
	return name == "c" || name == "c++"
}

// languageLabel returns the name shown in titles and toasts for the version
// browser, since the C entry is backed by Clang.
func (m SelectedInstalledModel) languageLabel() string {
	if isClang(m.language) {
		return "Clang"
	}
	if isGo(m.language) {
		return "Go"
	}
	return m.language.Name
}

func (m SelectedInstalledModel) Init() tea.Cmd {
	switch {
	case isGo(m.language):
		return tea.Batch(
			m.spinner.Init(),
			fetchGoVersionsCmd(),
		)
	case isClang(m.language):
		return tea.Batch(
			m.spinner.Init(),
			fetchClangVersionsCmd(),
		)
	default:
		return nil
	}
}

func fetchGoVersionsCmd() tea.Cmd {
	return func() tea.Msg {
		releases, err := api.ReleasesForMachine()
		return VersionsFetchedMsg{Releases: releases, Err: err}
	}
}

func fetchClangVersionsCmd() tea.Cmd {
	return func() tea.Msg {
		releases, err := api.FetchLLVMReleases()
		return VersionsFetchedMsg{Releases: releases, Err: err}
	}
}

func (m *SelectedInstalledModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	if m.width > 0 {
		barWidth := m.width / 4
		if barWidth > 32 {
			barWidth = 32
		}
		if barWidth < 10 {
			barWidth = 10
		}
		m.progress.SetBarWidth(barWidth)
	}

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
			if m.busy() {
				if m.cancel != nil {
					// Interrupt: cancel the download/pipeline. Nothing has
					// been swapped yet (swap happens only on success), so
					// backing out is always safe. The pipeline's result
					// message will restore the list state.
					m.cancel()
					return m, nil
				}
				// Safety net: busy state with no live pipeline (a dropped
				// result message). Reset so esc can never trap the user.
				m.phase = utils.PhaseIdle
				m.activeVersion = ""
				m.cancel = nil
				m.restoreList()
				utils.ClearFooterHint()
				return m, nil
			}
			if m.phase == utils.PhaseDone || m.phase == utils.PhaseFailed {
				// First esc after a finished install: return to the version
				// list view instead of leaving the screen. A second esc then
				// goes back to the previous menu.
				m.phase = utils.PhaseIdle
				m.err = nil
				m.activeVersion = ""
				m.restoreList()
				utils.ClearFooterHint()
				return m, nil
			}
			return m, func() tea.Msg { return utils.BackMsg{} }

		case "enter":
			if m.busy() {
				return m, nil
			}
			entry, ok := m.selectedEntry()
			if !ok {
				return m, nil
			}
			if entry.current {
				// Re-downloading the active version is pointless — say so in
				// orange, then let the footer fall back to its default hint.
				utils.EmitFooterHintColored(
					fmt.Sprintf("You already have this version (%s) • pick another or esc back", entry.title),
					"#FFA500")
				return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
					return footerHintResetMsg{}
				})
			}
			if entry.file.Filename != "" {
				// startDownload mutates m through its pointer receiver; the
				// updated value is what we return below. Note it must not
				// return a *SelectedInstalledModel as tea.Model — main.go
				// asserts the value type.
				cmd := m.startDownload(entry)
				return m, cmd
			}
			return m, nil
		}

	case footerHintResetMsg:
		utils.ClearFooterHint()
		return m, nil

	case VersionsFetchedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			m.SetSize(m.width, m.height)
			return m, nil
		}

		m.err = nil
		m.releases = msg.Releases
		m.entries = buildVersionEntries(m.language, m.currentVersion, msg.Releases)
		m.setItemsFromEntries()
		m.SetSize(m.width, m.height)
		return m, nil

	case utils.DownloadProgressMsg:
		if msg.Seq != m.seq {
			return m, m.nextProgress() // stale event from a cancelled run
		}
		switch msg.Phase {
		case utils.PhaseVerifying:
			m.phase = utils.PhaseVerifying
			return m, tea.Batch(m.progress.SetPercent(1.0), m.nextProgress())
		case utils.PhaseExtracting:
			m.phase = utils.PhaseExtracting
			return m, m.nextProgress()
		case utils.PhaseSwapping:
			m.phase = utils.PhaseSwapping
			return m, m.nextProgress()
		default:
			m.phase = utils.PhaseDownloading
			m.downloaded = msg.Downloaded
			m.total = msg.Total
			if msg.Percent >= 0 {
				m.lastPercent = msg.Percent
			}
			pct := msg.Percent
			if pct < 0 {
				pct = 0
			}
			return m, tea.Batch(m.progress.SetPercent(pct), m.nextProgress())
		}

	case utils.DownloadResultMsg:
		if msg.Seq != m.seq {
			return m, nil // stale result from a superseded run
		}
		m.cancel = nil
		switch msg.Phase {
		case utils.PhaseDone:
			// Swap completed: the new toolchain is active.
			m.phase = utils.PhaseDone
			m.activeVersion = msg.Version
			m.currentVersion = msg.Version
			m.entries = buildVersionEntries(m.language, m.currentVersion, m.releases)
			m.setItemsFromEntries()

			utils.EmitFooterHint(fmt.Sprintf(
				"%s %s installed • esc back to versions", m.languageLabel(), msg.Version))

			return m, m.progress.SetPercent(1.0)

		case utils.PhaseCancelled:
			// User interrupted: nothing was touched, restore the list.
			m.phase = utils.PhaseIdle
			m.activeVersion = ""
			m.restoreList()
			utils.EmitFooterHint("enter download • esc back")
			return m, nil

		default: // PhaseFailed
			m.phase = utils.PhaseFailed
			m.err = msg.Err
			m.restoreList()
			utils.EmitFooterHint("enter retry • esc back")
			return m, nil
		}

	case progressDrainDoneMsg:
		// Pipeline finished; stop listening for progress events.
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

	// Drive the progress bar animation frames.
	var progressCmd tea.Cmd
	m.progress, progressCmd = m.progress.Update(msg)
	cmds = append(cmds, progressCmd)

	return m, tea.Batch(cmds...)
}

func (m *SelectedInstalledModel) setItemsFromEntries() {
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
}

// restoreList brings back the full version list after a cancelled or failed
// download attempt.
func (m *SelectedInstalledModel) restoreList() {
	m.entries = buildVersionEntries(m.language, m.currentVersion, m.releases)
	m.setItemsFromEntries()
	m.progress.SetPercent(0)
	m.SetSize(m.width, m.height)
}

func (m SelectedInstalledModel) busy() bool {
	switch m.phase {
	case utils.PhaseDownloading, utils.PhaseVerifying, utils.PhaseExtracting, utils.PhaseSwapping:
		return true
	}
	return false
}

// Busy reports whether a download/install pipeline is currently running.
func (m SelectedInstalledModel) Busy() bool { return m.busy() }

// Cancel aborts an in-flight pipeline, if any. Safe to call when idle.
func (m SelectedInstalledModel) Cancel() {
	if m.cancel != nil {
		m.cancel()
	}
}

// startDownload launches the full pipeline for the selected version:
// download (with live progress) → SHA256 verify → extract → swap. It mutates
// the model in place and returns the initial command batch; progress events
// keep arriving as messages afterwards.
func (m *SelectedInstalledModel) startDownload(entry versionEntry) tea.Cmd {
	version := entry.release.Version
	filename := entry.file.Filename
	url := entry.file.URL
	if url == "" {
		url = "https://go.dev/dl/" + filename
	}
	sha := entry.file.SHA256

	m.phase = utils.PhaseDownloading
	m.activeVersion = version
	m.err = nil
	m.downloaded = 0
	m.total = entry.file.Size
	m.lastPercent = 0
	m.progress.SetPercent(0)

	// While downloading, the list is reduced to the version being
	// installed so the focus stays on it.
	m.entries = []versionEntry{entry}
	m.setItemsFromEntries()

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	m.seq++
	seq := m.seq

	ch := make(chan utils.DownloadProgressMsg, 64)
	done := make(chan struct{})
	m.progressCh = ch
	m.progressDone = done

	utils.EmitFooterHint(fmt.Sprintf("downloading %s • esc to cancel", version))

	downloadCmd := func() tea.Msg {
		defer close(done)

		dlErr := utils.DownloadFile(ctx, url, filepath.Join(utils.SwapFilesDir(), filename), sha, seq, ch)
		if dlErr != nil {
			if ctx.Err() != nil {
				return utils.DownloadResultMsg{Seq: seq, Phase: utils.PhaseCancelled, Version: version, Filename: filename, Err: dlErr}
			}
			return utils.DownloadResultMsg{Seq: seq, Phase: utils.PhaseFailed, Version: version, Filename: filename, Err: dlErr}
		}

		archivePath := filepath.Join(utils.SwapFilesDir(), filename)

		report := func(p utils.DownloadProgressMsg) {
			p.Seq = seq
			select {
			case ch <- p:
			default:
			}
		}

		var installDir string
		var instErr error
		if isClang(m.language) {
			installDir, instErr = utils.InstallClangArchive(ctx, archivePath, filename, report)
		} else {
			installDir, instErr = utils.InstallGoArchive(ctx, archivePath, filename, report)
		}
		if instErr != nil {
			if ctx.Err() != nil {
				return utils.DownloadResultMsg{Seq: seq, Phase: utils.PhaseCancelled, Version: version, Filename: filename, Err: instErr}
			}
			return utils.DownloadResultMsg{Seq: seq, Phase: utils.PhaseFailed, Version: version, Filename: filename, Err: instErr}
		}

		// The archive is installed; the compressed file is no longer needed.
		os.Remove(filepath.Join(utils.SwapFilesDir(), filename))

		return utils.DownloadResultMsg{
			Seq:       seq,
			Phase:     utils.PhaseDone,
			Version:   version,
			Filename:  filename,
			Archive:   installDir,
			BytesRead: entry.file.Size,
		}
	}

	return tea.Batch(downloadCmd, m.nextProgress())
}

// nextProgress returns a command that waits for the next progress event (or
// pipeline shutdown). Re-armed after every event is processed.
func (m *SelectedInstalledModel) nextProgress() tea.Cmd {
	if m.progressCh == nil {
		return nil
	}
	ch := m.progressCh
	done := m.progressDone

	return func() tea.Msg {
		select {
		case p := <-ch:
			return p
		case <-done:
			return progressDrainDoneMsg{}
		}
	}
}

// buildVersionEntries turns the fetched releases into list entries filtered
// to the current machine, with the currently installed version first.
func buildVersionEntries(lang utils.Language, currentOverride string, releases []api.Release) []versionEntry {
	current := currentOverride
	if current == "" {
		switch {
		case isGo(lang):
			current = extractGoVersion(lang.Version)
		case isClang(lang):
			current = extractClangVersion(lang.Version)
		}
	}

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

// extractClangVersion pulls the "X.Y.Z" version token out of a `clang
// --version` output such as "clang version 20.1.8" or
// "Ubuntu clang version 16.0.6".
func extractClangVersion(versionOutput string) string {
	for _, field := range strings.Fields(versionOutput) {
		if looksLikeVersion(field) {
			return field
		}
	}
	return ""
}

// looksLikeVersion reports whether s is a dotted numeric version such as
// "20.1.8" or "1.25.2".
func looksLikeVersion(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
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
				"No matching download found for %s/%s",
				api.MachineOS(), api.MachineArch())) + "\n")
		return b.String()
	}

	stability := "archived"
	if entry.release.Stable {
		stability = "stable release"
	}

	url := entry.file.URL
	if url == "" {
		url = "https://go.dev/dl/" + entry.file.Filename
	}
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

func (m SelectedInstalledModel) statusText() string {
	switch m.phase {
	case utils.PhaseDownloading:
		if m.total > 0 {
			return fmt.Sprintf("%s • %.0f%%", m.activeVersion, m.lastPercent*100)
		}
		return fmt.Sprintf("%s • %.1f MB", m.activeVersion, float64(m.downloaded)/(1024*1024))
	case utils.PhaseVerifying:
		return "verifying SHA256"
	case utils.PhaseExtracting:
		return "extracting"
	case utils.PhaseSwapping:
		return "swapping toolchain"
	case utils.PhaseDone:
		return "✓ " + m.activeVersion + " active"
	}
	return ""
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

	// Right side: animated download progress + status while active.
	var rightBlock string
	if m.busy() || m.phase == utils.PhaseDone {
		bar := m.progress.Bar()
		status := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Render(m.statusText())

		rightBlock = lipgloss.NewStyle().
			PaddingRight(2).
			PaddingLeft(1).
			Render(bar + " " + status)
	} else {
		rightBlock = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			PaddingRight(2).
			PaddingLeft(1).
			Render(fmt.Sprintf("Machine: [%s]", utils.GetMachineType()))
	}

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

	case m.phase == utils.PhaseDownloading || m.phase == utils.PhaseVerifying ||
		m.phase == utils.PhaseExtracting || m.phase == utils.PhaseSwapping:
		hint := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			PaddingLeft(2).
			Render(fmt.Sprintf("Installing %s — press esc to cancel (nothing is replaced until the swap completes)",
				m.activeVersion))
		content.WriteString(hint)
		content.WriteString("\n")

	case m.phase == utils.PhaseDone:
		hint := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ff00")).
			PaddingLeft(2).
			Render(fmt.Sprintf("✓ %s %s installed and set as the active %s toolchain",
				m.languageLabel(), m.activeVersion, m.languageLabel()))
		content.WriteString(hint)
		content.WriteString("\n")

		// Keep the version list visible under the success banner so the
		// screen never becomes a dead end; esc restores the normal title.
		if m.versions != nil {
			content.WriteString(m.versions.View())
		}
		content.WriteString("\n")

	case m.err != nil && m.phase == utils.PhaseFailed:
		hint := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff5555")).
			PaddingLeft(2).
			Render(fmt.Sprintf("Install failed: %v — press enter to retry, esc to go back", m.err))
		content.WriteString(hint)
		content.WriteString("\n")

		if m.versions != nil {
			content.WriteString(m.versions.View())
		}
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
			Render(fmt.Sprintf("Available %s Versions (%d)", m.languageLabel(), len(m.entries)))

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
