package utils

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abdimk/openvm/internals/api"

	ui "github.com/abdimk/openvm/cmd/ui"
)

type UpdateCheckModel struct {
	loading bool
	done    bool
	lines   []string
	pager   ui.Pager
	spinner ui.Spinner
}

func NewUpdateCheckModel() UpdateCheckModel {
	pager := ui.NewPager("Check For Update", "")
	pager.HideHeader()
	pager.HideFooter()
	return UpdateCheckModel{
		loading: true,
		pager:   pager,
		spinner: ui.SpinnerModel("Checking for updates..."),
	}
}

type UpdateCheckDoneMsg struct {
	Lines []string
}

func (m UpdateCheckModel) Init() tea.Cmd {
	work := func() tea.Msg {
		return UpdateCheckDoneMsg{Lines: updateCheckLines()}
	}
	return tea.Batch(m.spinner.Init(), work)
}

func (m UpdateCheckModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg { return BackMsg{} }
		}
	case UpdateCheckDoneMsg:
		m.loading = false
		m.done = true
		m.lines = msg.Lines
		m.pager.SetContent(m.renderLines())
	}

	var cmds []tea.Cmd

	if m.loading {
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	inner, _ := m.pager.Update(msg)
	m.pager = inner

	return m, tea.Batch(cmds...)
}

func (m *UpdateCheckModel) SetSize(width, height int) {
	m.pager.SetContent(m.renderLines())

	m.pager, _ = m.pager.Update(tea.WindowSizeMsg{
		Width:  width,
		Height: height,
	})
}

func (m UpdateCheckModel) renderLines() string {
	if !m.done {
		return waitingView()
	}
	var b strings.Builder
	for _, line := range m.lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return lipgloss.NewStyle().PaddingLeft(2).Render(strings.TrimSuffix(b.String(), "\n"))
}

func (m UpdateCheckModel) View() tea.View {
	if !m.done {
		return tea.NewView(lipgloss.NewStyle().PaddingLeft(2).Render(m.spinner.View()))
	}
	return tea.NewView(m.pager.View())
}

func updateCheckLines() []string {
	languages := GetAvailable()
	if len(languages) == 0 {
		return []string{"No supported languages or tools detected on this machine."}
	}

	lines := make([]string, 0, len(languages))
	for _, lang := range languages {
		lines = append(lines, updateCheckLine(lang))
	}
	return lines
}

func updateCheckLine(lang Language) string {
	current := firstSemverField(lang.Version)
	if current == "" {
		return fmt.Sprintf("[%s] installed %s — version could not be parsed", lang.Name, strings.TrimSpace(lang.Version))
	}

	latest, err := latestVersionFor(lang.Name)
	if err != nil {
		return fmt.Sprintf("[%s] %s installed • latest check failed: %v", lang.Name, current, err)
	}

	if semverGreater(latest, current) {
		return fmt.Sprintf("[%s] %s installed • latest %s • UPDATE AVAILABLE", lang.Name, current, latest)
	}
	return fmt.Sprintf("[%s] %s installed • latest %s • up to date", lang.Name, current, latest)
}

func latestVersionFor(name string) (string, error) {
	switch name {
	case Go:
		releases, err := api.FetchStableReleases()
		if err != nil {
			return "", err
		}
		if len(releases) == 0 {
			return "", fmt.Errorf("no Go releases returned")
		}
		return firstSemverField(releases[0].Version), nil
	case Python:
		releases, err := api.FetchPythonVersions()
		if err != nil {
			return "", err
		}
		if len(releases) == 0 {
			return "", fmt.Errorf("no Python releases returned")
		}
		return firstSemverField(releases[0].Version), nil
	case Node:
		releases, err := api.FetchNodeVersions()
		if err != nil {
			return "", err
		}
		if len(releases) == 0 {
			return "", fmt.Errorf("no Node releases returned")
		}
		return firstSemverField(releases[0].Version), nil
	case Rust:
		releases, err := api.FetchRustReleases()
		if err != nil {
			return "", err
		}
		if len(releases) == 0 {
			return "", fmt.Errorf("no Rust releases returned")
		}
		return firstSemverField(releases[0].Version), nil
	case Gpp, Gcc:
		releases, err := api.FetchLLVMReleases()
		if err != nil {
			return "", err
		}
		if len(releases) == 0 {
			return "", fmt.Errorf("no LLVM releases returned")
		}
		return firstSemverField(releases[0].Version), nil
	case Docker:
		releases, err := api.FetchDockerReleases()
		if err != nil {
			return "", err
		}
		if len(releases) == 0 {
			return "", fmt.Errorf("no Docker releases returned")
		}
		return firstSemverField(releases[0].Version), nil
	case Kubernetes:
		releases, err := api.FetchKubernetesReleases()
		if err != nil {
			return "", err
		}
		if len(releases) == 0 {
			return "", fmt.Errorf("no Kubernetes releases returned")
		}
		return firstSemverField(releases[0].Version), nil
	default:
		return "", fmt.Errorf("no update source for %s", name)
	}
}
