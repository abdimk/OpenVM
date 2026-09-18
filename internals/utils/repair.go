package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
)

type repairSpec struct {
	dir    string
	marker string
}

var repairSpecs = []repairSpec{
	{dir: "go", marker: "go"},
	{dir: "python", marker: "python3"},
	{dir: "node", marker: "node"},
	{dir: "rust", marker: "rustc"},
	{dir: "llvm", marker: "clang"},
	{dir: "docker", marker: "docker"},
	{dir: "kubectl", marker: "kubectl"},
}

func (s repairSpec) markerPath() string {
	name := s.marker
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if s.dir == "python" && runtime.GOOS != "windows" {
		return filepath.Join("bin", name)
	}
	if s.dir == "node" && runtime.GOOS != "windows" {
		return filepath.Join("bin", name)
	}
	if s.dir == "rust" || s.dir == "llvm" {
		return filepath.Join("bin", name)
	}
	return name
}

type RepairModel struct {
	loading bool
	done    bool
	lines   []string
	pager   ui.Pager
}

func NewRepairModel() RepairModel {
	pager := ui.NewPager("Repair", "")
	pager.ShowPlainHeader("Repair", MachineLabel())
	return RepairModel{
		loading: true,
		pager:   pager,
	}
}

type RepairDoneMsg struct {
	Lines []string
}

func (m RepairModel) Init() tea.Cmd {
	return func() tea.Msg {
		return RepairDoneMsg{Lines: repairLines()}
	}
}

func (m RepairModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg { return BackMsg{} }
		}
	case RepairDoneMsg:
		m.loading = false
		m.done = true
		m.lines = msg.Lines
		m.pager.SetContent(m.renderLines())
	}

	inner, _ := m.pager.Update(msg)
	m.pager = inner

	return m, nil
}

func (m *RepairModel) SetSize(width, height int) {
	m.pager.SetContent(m.renderLines())

	m.pager, _ = m.pager.Update(tea.WindowSizeMsg{
		Width:  width,
		Height: height,
	})
}

func (m RepairModel) renderLines() string {
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

func (m RepairModel) View() tea.View {
	return tea.NewView(m.pager.View())
}

func waitingView() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		PaddingLeft(2).
		Render("Checking managed toolchains...")
}

func repairLines() []string {
	root := SwapFilesDir()
	var lines []string

	stale, err := removeStaleDirs(root)
	if err == nil && stale > 0 {
		lines = append(lines, fmt.Sprintf("[cleanup] removed %d stale directory(ies)", stale))
	}

	for _, spec := range repairSpecs {
		toolDir := filepath.Join(root, spec.dir)
		info, err := os.Stat(toolDir)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			lines = append(lines, fmt.Sprintf("[%s] found a non-directory at %s", spec.dir, toolDir))
			continue
		}

		marker := filepath.Join(toolDir, spec.markerPath())
		if _, statErr := os.Stat(marker); statErr != nil {
			lines = append(lines, fmt.Sprintf("[%s] missing %s — reinstall recommended", spec.dir, spec.markerPath()))
			continue
		}

		if err := makeExecutable(marker); err != nil {
			lines = append(lines, fmt.Sprintf("[%s] could not restore execute permission: %v", spec.dir, err))
			continue
		}

		if err := ActivatePath(toolPathDir(spec.dir)); err != nil {
			lines = append(lines, fmt.Sprintf("[%s] PATH repair failed: %v", spec.dir, err))
			continue
		}

		lines = append(lines, fmt.Sprintf("[%s] OK — marker and PATH verified", spec.dir))
	}

	if len(lines) == 0 {
		lines = append(lines, "No managed toolchains found. Install something first.")
	}

	return lines
}

func removeStaleDirs(root string) (int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}

	removed := 0
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), "extract_") || strings.HasSuffix(e.Name(), ".bak") {
			names = append(names, filepath.Join(root, e.Name()))
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if err := os.RemoveAll(name); err == nil {
			removed++
		}
	}
	return removed, nil
}

func makeExecutable(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return os.Chmod(path, 0o755)
}
