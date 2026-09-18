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
	"github.com/abdimk/openvm/internals/swap"
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

type speedSample struct {
	at    time.Time
	bytes int64
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

	downloadSpeed float64
	speedSamples  []speedSample
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

func isDevTool(lang utils.Language) bool {
	return isDocker(lang) || isKubernetes(lang)
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
			if m.versions != nil && m.versions.SettingFilter() {
				return m, m.versions.Update(msg)
			}
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
			m.downloadSpeed = 0
			m.speedSamples = nil
			return m, tea.Batch(m.progress.SetPercent(1.0), m.nextProgress())
		case utils.PhaseExtracting:
			m.phase = utils.PhaseExtracting
			m.downloadSpeed = 0
			m.speedSamples = nil
			return m, m.nextProgress()
		case utils.PhaseSwapping:
			m.phase = utils.PhaseSwapping
			m.downloadSpeed = 0
			m.speedSamples = nil
			return m, m.nextProgress()
		default:
			m.phase = utils.PhaseDownloading
			m.downloaded = msg.Downloaded
			m.total = msg.Total
			m.recordSpeed(time.Now(), msg.Downloaded)
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

	if m.loading || m.busy() {
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
		if err == nil {
			_ = api.ResolveFileSize(&file)
		}
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

func (m *SelectedInstalledModel) recordSpeed(at time.Time, downloaded int64) {
	m.speedSamples = append(m.speedSamples, speedSample{at: at, bytes: downloaded})
	cutoff := at.Add(-3 * time.Second)
	for len(m.speedSamples) > 0 && m.speedSamples[0].at.Before(cutoff) {
		m.speedSamples = m.speedSamples[1:]
	}

	if len(m.speedSamples) >= 2 {
		first := m.speedSamples[0]
		last := m.speedSamples[len(m.speedSamples)-1]
		if dt := last.at.Sub(first.at).Seconds(); dt > 0 {
			m.downloadSpeed = float64(last.bytes-first.bytes) / dt
		}
	}
}

func formatBytesPerSec(bps float64) string {
	switch {
	case bps >= 1024*1024:
		return fmt.Sprintf("%.1f MB/s", bps/(1024*1024))
	case bps >= 1024:
		return fmt.Sprintf("%.0f KB/s", bps/1024)
	default:
		return fmt.Sprintf("%.0f B/s", bps)
	}
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
	m.downloadSpeed = 0
	m.speedSamples = nil
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
			case <-ctx.Done():
			}
		}

		var installDir string
		var instErr error
		switch {
		case isClang(m.language):
			installDir, instErr = swap.InstallClangArchive(ctx, archivePath, filename, report)
		case isPython(m.language):
			installDir, instErr = swap.InstallPythonArchive(ctx, archivePath, filename, report)
		case isRust(m.language):
			installDir, instErr = swap.InstallRustArchive(ctx, archivePath, filename, report)
		case isNode(m.language):
			installDir, instErr = swap.InstallNodeArchive(ctx, archivePath, filename, report)
		case isDocker(m.language):
			installDir, instErr = swap.InstallDockerArchive(ctx, archivePath, filename, report)
		case isKubernetes(m.language):
			installDir, instErr = swap.InstallKubectlBinary(ctx, archivePath, filename, report)
		default:
			installDir, instErr = swap.InstallGoArchive(ctx, archivePath, filename, report)
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

	return tea.Batch(m.spinner.Init(), downloadCmd, m.nextProgress())
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
			select {
			case p := <-ch:
				return p
			default:
				return progressDrainDoneMsg{}
			}
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

func (m SelectedInstalledModel) installInfoView() string {
	pkg := fmt.Sprintf("%s %s", m.languageLabel(), m.activeVersion)

	green := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00ff00"))
	gold := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A"))
	faint := lipgloss.NewStyle().Foreground(lipgloss.Color("#5A5A5A"))
	white := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#D6D6D6"))

	spin := func() string {
		return strings.TrimSpace(m.spinner.WithText("").View())
	}

	action := "downloading"
	switch m.phase {
	case utils.PhaseVerifying:
		action = "verifying checksum for"
	case utils.PhaseExtracting:
		action = "extracting"
	case utils.PhaseSwapping:
		action = "installing"
	}

	var b strings.Builder

	row := func(left, right string) {
		gap := 2
		if m.width > 0 {
			gap = m.width - lipgloss.Width(left) - lipgloss.Width(right)
			if gap < 2 {
				gap = 2
			}
		}
		b.WriteString(left + strings.Repeat(" ", gap) + right + "\n")
	}

	row(gold.Render(spin())+"  "+white.Render(action)+" "+gold.Render(pkg), "")

	steps := []struct{ done, active string }{
		{"Checksum verified", "Verifying checksum"},
		{"Archive extracted", "Extracting files"},
		{"Toolchain installed", "Installing toolchain"},
	}

	dlMarker, dlLabel := dim.Render("·"), faint.Render("Downloaded archive")
	if m.phase == utils.PhaseDownloading {
		dlMarker, dlLabel = gold.Render("●"), gold.Render("Downloading archive")
	} else {
		dlMarker, dlLabel = green.Render("✓"), white.Render("Downloaded archive")
	}

	dlRight := ""
	if m.phase == utils.PhaseDownloading {
		mb := float64(m.downloaded) / (1024 * 1024)
		if m.total > 0 {
			dlRight = fmt.Sprintf("%.1f / %.1f MB", mb, float64(m.total)/(1024*1024))
		} else {
			dlRight = fmt.Sprintf("%.1f MB", mb)
		}
		dlRight = dim.Render(dlRight)
	}
	row("  "+dlMarker+"  "+dlLabel, dlRight)

	for i, s := range steps {
		marker, label := dim.Render("·"), faint.Render(s.done)
		switch m.phase {
		case utils.PhaseVerifying:
			if i == 0 {
				marker, label = gold.Render("●"), gold.Render(s.active)
			}
		case utils.PhaseExtracting:
			if i == 0 {
				marker, label = green.Render("✓"), white.Render(s.done)
			} else if i == 1 {
				marker, label = gold.Render("●"), gold.Render(s.active)
			}
		case utils.PhaseSwapping:
			if i <= 1 {
				marker, label = green.Render("✓"), white.Render(s.done)
			} else if i == 2 {
				marker, label = gold.Render("●"), gold.Render(s.active)
			}
		}
		row("  "+marker+"  "+label, "")
	}

	b.WriteString("\n")

	speedText := ""
	if m.phase == utils.PhaseDownloading && len(m.speedSamples) >= 2 {
		speedText = dim.Render(formatBytesPerSec(m.downloadSpeed))
	}
	row("  "+m.progress.Bar(), speedText)
	b.WriteString("\n")

	b.WriteString("  " + faint.Render(
		"press esc to cancel — nothing is replaced until the swap completes"))

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
		label := "Language"
		if isDevTool(m.language) {
			label = "DevTool"
		}
		return fmt.Sprintf("%s: [%s]", label, m.language.Name)
	}

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff"))

	label := "Language:"
	if isDevTool(m.language) {
		label = "DevTool:"
	}

	leftBlock := labelStyle.
		PaddingLeft(2).
		PaddingBottom(0).
		Render(label)

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
		status := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Render(m.statusText())

		rightBlock = lipgloss.NewStyle().
			PaddingRight(2).
			PaddingLeft(1).
			Render(status)
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
		content.WriteString(m.installInfoView())
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
