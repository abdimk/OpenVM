package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
)

type DoctorModel struct {
	loading bool
	done    bool
	lines   []string
	pager   ui.Pager
	spinner ui.Spinner
}

func NewDoctorModel() DoctorModel {
	pager := ui.NewPager("Doctor", "")
	pager.ShowPlainHeader("Doctor", machineLabel())
	return DoctorModel{
		loading: true,
		pager:   pager,
		spinner: ui.SpinnerModel("Running diagnostics..."),
	}
}

type DoctorDoneMsg struct {
	Lines []string
}

func (m DoctorModel) Init() tea.Cmd {
	work := func() tea.Msg {
		return DoctorDoneMsg{Lines: doctorLines()}
	}
	return tea.Batch(m.spinner.Init(), work)
}

func (m DoctorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg { return BackMsg{} }
		}
	case DoctorDoneMsg:
		m.loading = false
		m.done = true
		m.lines = msg.Lines
		m.pager.SetContent(m.renderLines())
	}

	var cmds []tea.Cmd

	if m.loading {
		m.pager.SetContent(m.renderLines())
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	inner, _ := m.pager.Update(msg)
	m.pager = inner

	return m, tea.Batch(cmds...)
}

func (m *DoctorModel) SetSize(width, height int) {
	m.pager.SetContent(m.renderLines())

	m.pager, _ = m.pager.Update(tea.WindowSizeMsg{
		Width:  width,
		Height: height,
	})
}

func (m DoctorModel) renderLines() string {
	if !m.done {
		return lipgloss.NewStyle().PaddingLeft(2).Render(m.spinner.View())
	}
	var b strings.Builder
	for _, line := range m.lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return lipgloss.NewStyle().PaddingLeft(2).Render(strings.TrimSuffix(b.String(), "\n"))
}

func (m DoctorModel) View() tea.View {
	return tea.NewView(m.pager.View())
}

func doctorLines() []string {
	var lines []string

	exe, err := os.Executable()
	if err != nil {
		lines = append(lines, fmt.Sprintf("[err] could not locate the OpenVM executable: %v", err))
	} else {
		lines = append(lines, fmt.Sprintf("[ok] OpenVM executable: %s", exe))
	}

	swapDir := SwapFilesDir()
	if fi, statErr := os.Stat(swapDir); statErr == nil && fi.IsDir() {
		lines = append(lines, fmt.Sprintf("[ok] toolchain directory: %s", swapDir))
	} else {
		lines = append(lines, fmt.Sprintf("[warn] toolchain directory missing: %s", swapDir))
	}

	if free := availableSpaceBytes(); free > 0 {
		lines = append(lines, fmt.Sprintf("[ok] free disk space: %s", humanBytes(free)))
	} else {
		lines = append(lines, "[warn] could not determine free disk space")
	}

	for _, tool := range installedToolchains() {
		binDir := filepath.Join(swapDir, toolPathDir(tool))
		if pathContainsDir(binDir) {
			lines = append(lines, fmt.Sprintf("[ok] %s binaries are on PATH (%s)", tool, binDir))
		} else {
			lines = append(lines, fmt.Sprintf("[warn] %s binaries not activated on PATH (%s)", tool, binDir))
		}
	}

	lines = append(lines, "[network]")
	for _, u := range []string{
		"https://go.dev/dl/?mode=json",
		"https://www.python.org/ftp/python/",
		"https://nodejs.org/dist/",
		"https://static.rust-lang.org/dist/channel-rust-stable.toml",
		"https://github.com/",
		"https://download.docker.com/",
		"https://dl.k8s.io/release/stable.txt",
	} {
		if err := probeURL(u); err != nil {
			lines = append(lines, fmt.Sprintf("[warn] %s unreachable: %v", u, err))
		} else {
			lines = append(lines, fmt.Sprintf("[ok] %s reachable", u))
		}
	}

	return lines
}
