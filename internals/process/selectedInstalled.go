package process

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
	"github.com/abdimk/openvm/internals/api"
	"github.com/abdimk/openvm/internals/utils"
)

type progressDrainDoneMsg struct{}

type footerHintResetMsg struct{}

type VersionsFetchedMsg struct {
	Releases []api.Release
	Err      error
}

type ArtifactResolvedMsg struct {
	Entry versionEntry
	File  api.File
	Err   error
}

type versionEntry struct {
	title   string
	release api.Release
	file    api.File
	current bool
}

type artifactFn func(version string) (api.File, error)

type SelectedInstalledModel struct {
	language utils.Language
	width    int
	height   int

	versions *ui.ListModel
	entries  []versionEntry
	releases []api.Release

	loading bool
	err     error

	resolving bool

	spinner  ui.Spinner
	progress ui.Progress

	resolveArtifact artifactFn

	phase          utils.DownloadPhase
	activeVersion  string
	lastPercent    float64
	downloaded     int64
	total          int64
	cancel         context.CancelFunc
	progressCh     <-chan utils.DownloadProgressMsg
	progressDone   <-chan struct{}
	seq            int
	currentVersion string
}

func NewSelectedInstalledModel(lang utils.Language) SelectedInstalledModel {
	m := SelectedInstalledModel{
		language: lang,
		loading:  isGo(lang) || isClang(lang) || isPython(lang) || isNode(lang) || isRust(lang) || isDocker(lang) || isKubernetes(lang),
		spinner:  ui.SpinnerModel("Fetching available versions..."),
		progress: ui.NewProgress(),
		phase:    utils.PhaseIdle,
	}
	switch {
	case isGo(m.language):
		m.resolveArtifact = func(version string) (api.File, error) {
			releases, err := api.ReleasesForMachine()
			if err != nil {
				return api.File{}, err
			}
			for _, r := range releases {
				if r.Version != version {
					continue
				}
				if file, ok := r.FileForMachine(); ok {
					return file, nil
				}
				break
			}
			return api.File{}, fmt.Errorf("no downloadable artifact for Go %s", version)
		}
	case isClang(m.language):
		m.resolveArtifact = func(version string) (api.File, error) {
			releases, err := api.FetchLLVMReleases()
			if err != nil {
				return api.File{}, err
			}
			for _, r := range releases {
				if r.Version != version {
					continue
				}
				if file, ok := r.FileForMachine(); ok {
					return file, nil
				}
				break
			}
			return api.File{}, fmt.Errorf("no downloadable artifact for LLVM %s", version)
		}
	case isPython(m.language):
		m.resolveArtifact = func(version string) (api.File, error) {
			return api.FetchPythonArtifact(version)
		}
	case isRust(m.language):
		m.resolveArtifact = func(version string) (api.File, error) {
			return api.FetchRustArtifact(version)
		}
	case isNode(m.language):
		m.resolveArtifact = func(version string) (api.File, error) {
			return api.FetchNodeArtifact(version)
		}
	case isDocker(m.language):
		m.resolveArtifact = func(version string) (api.File, error) {
			return api.FetchDockerArtifact(version)
		}
	case isKubernetes(m.language):
		m.resolveArtifact = func(version string) (api.File, error) {
			return api.FetchKubernetesArtifact(version)
		}
	}
	m.versions = ui.New("", nil, 80, 20)
	return m
}

func isGo(lang utils.Language) bool {
	return strings.EqualFold(lang.Name, utils.Go)
}

func isClang(lang utils.Language) bool {
	name := strings.ToLower(strings.TrimSpace(lang.Name))
	return name == "c" || name == "c++"
}

func isPython(lang utils.Language) bool {
	return strings.EqualFold(strings.TrimSpace(lang.Name), utils.Python)
}

func isRust(lang utils.Language) bool {
	return strings.EqualFold(strings.TrimSpace(lang.Name), utils.Rust)
}

func isNode(lang utils.Language) bool {
	return strings.EqualFold(strings.TrimSpace(lang.Name), utils.Node)
}

func isDocker(lang utils.Language) bool {
	return strings.EqualFold(strings.TrimSpace(lang.Name), utils.Docker)
}

func isKubernetes(lang utils.Language) bool {
	return strings.EqualFold(strings.TrimSpace(lang.Name), utils.Kubernetes)
}

func (m SelectedInstalledModel) languageLabel() string {
	if isClang(m.language) {
		return "Clang"
	}
	if isGo(m.language) {
		return "Go"
	}
	if isPython(m.language) {
		return "Python"
	}
	if isNode(m.language) {
		return "Node"
	}
	return m.language.Name
}

func (m SelectedInstalledModel) Init() tea.Cmd {
	cmd := m.fetchVersionsCmd()
	if cmd == nil {
		return nil
	}
	return tea.Batch(m.spinner.Init(), cmd)
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

func fetchRustVersionsCmd() tea.Cmd {
	return func() tea.Msg {
		releases, err := api.FetchRustReleases()
		return VersionsFetchedMsg{Releases: releases, Err: err}
	}
}

func (m SelectedInstalledModel) fetchVersionsCmd() tea.Cmd {
	switch {
	case isGo(m.language):
		return fetchGoVersionsCmd()
	case isClang(m.language):
		return fetchClangVersionsCmd()
	case isPython(m.language):
		return fetchPythonVersionsCmd()
	case isRust(m.language):
		return fetchRustVersionsCmd()
	case isNode(m.language):
		return fetchNodeVersionsCmd()
	case isDocker(m.language):
		return fetchDockerVersionsCmd()
	case isKubernetes(m.language):
		return fetchKubernetesVersionsCmd()
	default:
		return nil
	}
}

func fetchNodeVersionsCmd() tea.Cmd {
	return func() tea.Msg {
		infos, err := api.FetchNodeVersions()
		releases := make([]api.Release, 0, len(infos))
		for _, info := range infos {
			releases = append(releases, api.Release{Version: info.Version, Stable: true, Date: info.Date, LTS: info.LTS})
		}
		return VersionsFetchedMsg{Releases: releases, Err: err}
	}
}

func fetchDockerVersionsCmd() tea.Cmd {
	return func() tea.Msg {
		releases, err := api.FetchDockerReleases()
		return VersionsFetchedMsg{Releases: releases, Err: err}
	}
}

func fetchKubernetesVersionsCmd() tea.Cmd {
	return func() tea.Msg {
		releases, err := api.FetchKubernetesReleases()
		return VersionsFetchedMsg{Releases: releases, Err: err}
	}
}

func fetchPythonVersionsCmd() tea.Cmd {
	return func() tea.Msg {
		infos, err := api.FetchPythonVersions()
		releases := make([]api.Release, 0, len(infos))
		for _, info := range infos {
			releases = append(releases, api.Release{Version: info.Version, Stable: true, Date: info.Date})
		}
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

					m.cancel()
					return m, nil
				}

				m.phase = utils.PhaseIdle
				m.activeVersion = ""
				m.cancel = nil
				m.restoreList()
				utils.ClearFooterHint()
				return m, nil
			}
			m.resolving = false
			if m.phase == utils.PhaseDone || m.phase == utils.PhaseFailed {

				m.phase = utils.PhaseIdle
				m.err = nil
				m.activeVersion = ""
				m.restoreList()
				utils.ClearFooterHint()
				return m, nil
			}
			return m, func() tea.Msg { return utils.BackMsg{} }

		case "r":
			if m.busy() || m.resolving || m.loading || m.phase != utils.PhaseIdle {
				return m, nil
			}
			if m.err == nil {
				return m, nil
			}
			m.err = nil
			m.loading = true
			return m, tea.Batch(m.spinner.Init(), m.fetchVersionsCmd())

		case "enter":
			if m.busy() {
				return m, nil
			}
			entry, ok := m.selectedEntry()
			if !ok {
				return m, nil
			}
			if entry.current {

				utils.EmitFooterHintColored(
					fmt.Sprintf("You already have this version (%s) • pick another or esc back", entry.title),
					"#FFA500")
				return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
					return footerHintResetMsg{}
				})
			}
			if entry.file.Filename != "" {

				cmd := m.startDownload(entry)
				return m, cmd
			}
			if m.resolveArtifact == nil {
				return m, nil
			}
			return m, m.resolveEntryArtifact(entry)
		}

	case footerHintResetMsg:
		utils.ClearFooterHint()
		return m, nil

	case ArtifactResolvedMsg:
		if !m.resolving {
			return m, nil
		}
		m.resolving = false
		if msg.Err != nil {
			utils.EmitFooterHintColored(fmt.Sprintf("%v — esc back", msg.Err), "#ff5555")
			return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg {
				return footerHintResetMsg{}
			})
		}
		msg.Entry.file = msg.File
		for i := range m.entries {
			if m.entries[i].title == msg.Entry.title {
				m.entries[i].file = msg.File
				break
			}
		}
		m.setItemsFromEntries()
		m.SetSize(m.width, m.height)
		return m, m.startDownload(msg.Entry)

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
			return m, m.nextProgress()
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
			return m, nil
		}
		m.cancel = nil
		switch msg.Phase {
		case utils.PhaseDone:

			m.phase = utils.PhaseDone
			m.activeVersion = msg.Version
			m.currentVersion = msg.Version
			m.entries = buildVersionEntries(m.language, m.currentVersion, m.releases)
			m.setItemsFromEntries()

			utils.EmitFooterHint(fmt.Sprintf(
				"%s %s installed • esc back to versions", m.languageLabel(), msg.Version))

			return m, m.progress.SetPercent(1.0)

		case utils.PhaseCancelled:

			m.phase = utils.PhaseIdle
			m.activeVersion = ""
			m.restoreList()
			utils.EmitFooterHint("enter download • esc back")
			return m, nil

		default:
			m.phase = utils.PhaseFailed
			m.err = msg.Err
			m.restoreList()
			utils.EmitFooterHint("enter retry • esc back")
			return m, nil
		}

	case progressDrainDoneMsg:

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

	var progressCmd tea.Cmd
	m.progress, progressCmd = m.progress.Update(msg)
	cmds = append(cmds, progressCmd)

	return m, tea.Batch(cmds...)
}

func (m *SelectedInstalledModel) resolveEntryArtifact(entry versionEntry) tea.Cmd {
	m.resolving = true
	version := entry.title
	resolve := m.resolveArtifact
	utils.EmitFooterHint(fmt.Sprintf("resolving %s download...", version))

	return func() tea.Msg {
		file, err := resolve(version)
		return ArtifactResolvedMsg{Entry: entry, File: file, Err: err}
	}
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

func (m SelectedInstalledModel) Busy() bool { return m.busy() }

func (m SelectedInstalledModel) Cancel() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *SelectedInstalledModel) startDownload(entry versionEntry) tea.Cmd {
	version := entry.release.Version
	filename := entry.file.Filename
	url := entry.file.URL
	if url == "" {
		if !isGo(m.language) {
			return func() tea.Msg {
				return utils.DownloadResultMsg{
					Seq:      m.seq,
					Phase:    utils.PhaseFailed,
					Version:  version,
					Filename: filename,
					Err:      fmt.Errorf("no download URL available for %s", filename),
				}
			}
		}
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
		switch {
		case isClang(m.language):
			installDir, instErr = utils.InstallClangArchive(ctx, archivePath, filename, report)
		case isPython(m.language):
			installDir, instErr = utils.InstallPythonArchive(ctx, archivePath, filename, report)
		case isRust(m.language):
			installDir, instErr = utils.InstallRustArchive(ctx, archivePath, filename, report)
		case isNode(m.language):
			installDir, instErr = utils.InstallNodeArchive(ctx, archivePath, filename, report)
		case isDocker(m.language):
			installDir, instErr = utils.InstallDockerArchive(ctx, archivePath, filename, report)
		case isKubernetes(m.language):
			installDir, instErr = utils.InstallKubectlBinary(ctx, archivePath, filename, report)
		default:
			installDir, instErr = utils.InstallGoArchive(ctx, archivePath, filename, report)
		}
		if instErr != nil {
			if ctx.Err() != nil {
				return utils.DownloadResultMsg{Seq: seq, Phase: utils.PhaseCancelled, Version: version, Filename: filename, Err: instErr}
			}
			return utils.DownloadResultMsg{Seq: seq, Phase: utils.PhaseFailed, Version: version, Filename: filename, Err: instErr}
		}

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

func buildVersionEntries(lang utils.Language, currentOverride string, releases []api.Release) []versionEntry {
	current := currentOverride
	if current == "" {
		switch {
		case isGo(lang):
			current = currentGoVersion(lang)
		case isClang(lang):
			current = currentClangVersion(lang)
		case isPython(lang):
			current = currentPythonVersion(lang)
		case isRust(lang):
			current = currentRustVersion(lang)
		case isNode(lang):
			current = currentNodeVersion(lang)
		case isDocker(lang):
			current = currentDockerVersion(lang)
		case isKubernetes(lang):
			current = currentKubernetesVersion(lang)
		}
	}

	entries := make([]versionEntry, 0, len(releases)+1)
	currentIdx := -1

	for _, r := range releases {
		e := versionEntry{
			title:   r.Version,
			release: r,
			current: current != "" && r.Version == current,
		}
		if file, ok := r.FileForMachine(); ok {
			e.file = file
		}
		if e.current {
			currentIdx = len(entries)
		}
		entries = append(entries, e)
	}

	if current != "" && currentIdx < 0 {
		entries = append([]versionEntry{{
			title:   current,
			release: api.Release{Version: current},
			current: true,
		}}, entries...)
		currentIdx = 0
	}

	if currentIdx > 0 {
		e := entries[currentIdx]
		copy(entries[1:currentIdx+1], entries[0:currentIdx])
		entries[0] = e
	}

	return entries
}

func extractGoVersion(versionOutput string) string {
	for _, field := range strings.Fields(versionOutput) {
		if strings.HasPrefix(field, "go1.") {
			return field
		}
	}
	return ""
}

func extractClangVersion(versionOutput string) string {
	return extractDottedVersion(versionOutput)
}

func extractPythonVersion(versionOutput string) string {
	return extractDottedVersion(versionOutput)
}

func extractRustVersion(versionOutput string) string {
	return extractDottedVersion(versionOutput)
}

func currentRustVersion(lang utils.Language) string {
	if managed := utils.ManagedRustVersion(); managed != "" {
		if v := extractRustVersion(managed); v != "" {
			return v
		}
	}
	return extractRustVersion(lang.Version)
}

func extractNodeVersion(versionOutput string) string {
	for _, field := range strings.Fields(versionOutput) {
		bare := strings.TrimPrefix(field, "v")
		if looksLikeVersion(bare) {
			return bare
		}
	}
	return ""
}

func extractDockerVersion(versionOutput string) string {
	for _, field := range strings.Fields(versionOutput) {
		bare := strings.Trim(field, ",;:()")
		if looksLikeVersion(bare) {
			return bare
		}
	}
	return ""
}

func currentDockerVersion(lang utils.Language) string {
	if managed := utils.ManagedToolchainVersion("docker"); managed != "" {
		if v := extractDockerVersion(managed); v != "" {
			return v
		}
	}
	return extractDockerVersion(lang.Version)
}

func currentGoVersion(lang utils.Language) string {
	if managed := utils.ManagedToolchainVersion("go"); managed != "" {
		if v := extractGoVersion(managed); v != "" {
			return v
		}
	}
	return extractGoVersion(lang.Version)
}

func currentClangVersion(lang utils.Language) string {
	if managed := utils.ManagedClangVersion(); managed != "" {
		if v := extractClangVersion(managed); v != "" {
			return v
		}
	}
	return extractClangVersion(lang.Version)
}

func currentPythonVersion(lang utils.Language) string {
	if managed := utils.ManagedToolchainVersion("python"); managed != "" {
		if v := extractPythonVersion(managed); v != "" {
			return v
		}
	}
	return extractPythonVersion(lang.Version)
}

func currentNodeVersion(lang utils.Language) string {
	if managed := utils.ManagedToolchainVersion("node"); managed != "" {
		if v := extractNodeVersion(managed); v != "" {
			return v
		}
	}
	return extractNodeVersion(lang.Version)
}

var kubernetesVersionOutputRe = regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+`)

func extractKubernetesVersion(versionOutput string) string {
	return kubernetesVersionOutputRe.FindString(versionOutput)
}

func currentKubernetesVersion(lang utils.Language) string {
	if managed := utils.ManagedToolchainVersion("kubectl"); managed != "" {
		if v := extractKubernetesVersion(managed); v != "" {
			return v
		}
	}
	return extractKubernetesVersion(lang.Version)
}

func extractDottedVersion(versionOutput string) string {
	for _, field := range strings.Fields(versionOutput) {
		if looksLikeVersion(field) {
			return field
		}
	}
	return ""
}

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
		if e.current {
			return "Currently installed on this machine"
		}

		var parts []string
		if e.release.Stable {
			parts = append(parts, "stable")
		}
		if e.release.LTS != "" {
			parts = append(parts, "LTS "+e.release.LTS)
		}
		if e.release.Date != "" {
			parts = append(parts, e.release.Date)
		}
		if len(parts) == 0 {
			parts = append(parts, "archived")
		}
		return strings.Join(parts, " • ")
	}

	stability := "archived"
	if e.release.Stable {
		stability = "stable"
	}

	size := fmt.Sprintf("%.2f MB", e.file.SizeMB())
	if e.file.Size <= 0 {
		size = "size n/a"
	}

	return fmt.Sprintf("%s • %s/%s • %s • %s",
		stability, e.file.OS, e.file.Arch, e.file.Filename, size)
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
		if entry.current {
			b.WriteString(label.Render(" Path:     ") + value.Render(m.language.Path) + "\n")
			b.WriteString(label.Render(" Status:   ") +
				value.Render("Currently installed on this machine") + "\n")
			return b.String()
		}
		b.WriteString(label.Render(" Status:   ") +
			value.Render("Press enter to resolve the download for this version") + "\n")
		return b.String()
	}

	stability := "archived"
	if entry.release.Stable {
		stability = "stable release"
	}

	url := entry.file.URL
	if url == "" && isGo(m.language) {
		url = "https://go.dev/dl/" + entry.file.Filename
	}
	if url == "" {
		return b.String()
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
	size := fmt.Sprintf("%.2f MB", entry.file.SizeMB())
	if entry.file.Size <= 0 {
		size = "size n/a"
	}
	b.WriteString(label.Render(" Size:     ") +
		value.Render(size) + "\n")
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

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff"))

	leftBlock := labelStyle.
		PaddingLeft(2).
		PaddingBottom(0).
		Render("Language:")

	langStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ff00")).
		Render(m.language.Name)

	langPart := lipgloss.JoinHorizontal(
		lipgloss.Top,
		labelStyle.Render(" ["),
		langStyle,
		labelStyle.Render("]"),
	)

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
			PaddingRight(2).
			PaddingLeft(1).
			Render(utils.MachineLabel())
	}

	leftW := lipgloss.Width(leftBlock) + lipgloss.Width(langPart)
	rightW := lipgloss.Width(rightBlock)

	gap := m.width - leftW - rightW
	if gap < 0 {
		gap = 0
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftBlock,
		langPart,
		strings.Repeat(" ", gap),
		rightBlock,
	)
}

func (m SelectedInstalledModel) View() tea.View {
	var content strings.Builder

	content.WriteString(m.buildHeader())
	content.WriteString("\n\n")

	if isDocker(m.language) {
		if bundle, err := api.DockerBundleForMachine(runtime.GOOS, runtime.GOARCH); err == nil {
			bundleStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFD700")).
				PaddingLeft(2).
				PaddingBottom(1)
			content.WriteString(bundleStyle.Render(fmt.Sprintf("%s — includes: %s", bundle.Label, strings.Join(bundle.Components, ", "))))
			content.WriteString("\n")
			content.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				PaddingLeft(2).
				PaddingBottom(1).
				Render(bundle.Notice))
			content.WriteString("\n\n")
		}
	}

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
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Render("Check your network connection and press r to retry, esc to go back."))
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
