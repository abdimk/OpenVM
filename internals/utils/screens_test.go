package utils

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	ui "github.com/abdimk/openvm/cmd/ui"
)

func TestFirstSemverField(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"go1.24.3", "1.24.3"},
		{"go version go1.25.2 windows/amd64", "1.25.2"},
		{"Python 3.13.2", "3.13.2"},
		{"v22.13.1", "22.13.1"},
		{"node v22.13.1", "22.13.1"},
		{"rustc 1.85.1 (4d91de4e4 2024-12-10)", "1.85.1"},
		{"gcc (GCC) 14.2.0", "14.2.0"},
		{"Docker version 28.4.0, build 8a", "28.4.0"},
		{"Client Version: v1.31.0+k3s1", "1.31.0"},
		{"no numbers here", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := firstSemverField(c.in); got != c.want {
			t.Errorf("firstSemverField(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSemverGreater(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"1.25.2", "1.24.3", true},
		{"1.24.3", "1.25.2", false},
		{"2.0.0", "1.9.9", true},
		{"1.10.0", "1.9.9", true},
		{"1.9.9", "1.10.0", false},
		{"1.24.3", "1.24.3", false},
	}
	for _, c := range cases {
		if got := semverGreater(c.latest, c.current); got != c.want {
			t.Errorf("semverGreater(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestUpdateCheckLineFormatting(t *testing.T) {
	line := updateCheckLine(Language{Name: "Go", Version: "go1.24.3"})
	if !strings.HasPrefix(line, "[Go]") {
		t.Errorf("expected [Go] prefix, got %q", line)
	}
}

func TestUpdateCheckSpinnerWhileLoading(t *testing.T) {
	m := NewUpdateCheckModel()
	if v := m.View().Content; strings.TrimSpace(v) == "" {
		t.Errorf("expected a spinner view while loading")
	}
}

func TestUpdateCheckStopsSpinnerAfterDone(t *testing.T) {
	m := NewUpdateCheckModel()
	upd, cmd := m.Update(UpdateCheckDoneMsg{Lines: []string{"[Go] up to date"}})
	if cmd != nil {
		t.Errorf("expected no command after done, got %v", cmd)
	}
	if v := upd.(interface{ View() tea.View }).View().Content; strings.TrimSpace(v) == "" {
		t.Errorf("expected content view after done")
	}
}

func TestScreenModelsRenderAfterDone(t *testing.T) {
	update := NewUpdateCheckModel()
	repair := NewRepairModel()
	doctor := NewDoctorModel()
	current := NewCurrentVersionModel()

	models := []tea.Model{&update, &repair, &doctor, &current}
	doneMsgs := []tea.Msg{
		UpdateCheckDoneMsg{Lines: []string{"[Go] up to date"}},
		RepairDoneMsg{Lines: []string{"[go] OK"}},
		DoctorDoneMsg{Lines: []string{"[ok] checked"}},
		CurrentVersionDoneMsg{Lines: []string{"[Go] 1.24.3"}},
	}

	for i, model := range models {
		model.Update(doneMsgs[i])
		sized := model.(interface{ SetSize(width, height int) })
		sized.SetSize(100, 30)
		if v := model.(interface{ View() tea.View }).View().Content; strings.TrimSpace(v) == "" {
			t.Errorf("model %d: expected non-empty view after done", i)
		}
	}
}

func TestRepairLinesAreDeterministic(t *testing.T) {
	lines := repairLines()
	if len(lines) == 0 {
		t.Fatal("repairLines returned nothing")
	}
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			t.Errorf("empty repair line")
		}
	}
}

func TestToolPathDir(t *testing.T) {
	if got := toolPathDir("docker"); got != "docker" {
		t.Errorf("toolPathDir(docker) = %q, want docker", got)
	}
	if got := toolPathDir("llvm"); got != filepath.Join("llvm", "bin") {
		t.Errorf("toolPathDir(llvm) = %q, want %q", got, filepath.Join("llvm", "bin"))
	}
}

func TestRepairMarkerPaths(t *testing.T) {
	for _, spec := range repairSpecs {
		p := spec.markerPath()
		if p == "" {
			t.Errorf("empty marker for %s", spec.dir)
		}
	}
}

func TestNameToToolDir(t *testing.T) {
	cases := []struct {
		name   string
		want   string
		wantOK bool
	}{
		{Go, "go", true},
		{Python, "python", true},
		{Node, "node", true},
		{Rust, "rust", true},
		{Gpp, "llvm", true},
		{Gcc, "llvm", true},
		{Docker, "docker", true},
		{Kubernetes, "kubectl", true},
		{"NotARealTool", "", false},
	}
	for _, c := range cases {
		dir, ok := nameToToolDir(c.name)
		if dir != c.want || ok != c.wantOK {
			t.Errorf("nameToToolDir(%q) = %q, %v; want %q, %v", c.name, dir, ok, c.want, c.wantOK)
		}
	}
}

func TestInstalledConfirmView(t *testing.T) {
	m := NewInstalledModel()
	m.langs = []Language{{Name: Go, Version: "go1.24.3", Path: "go"}}
	m.languages = ui.New("", []ui.Item{{TitleText: Go, DescriptionText: "go1.24.3"}}, 80, 20)
	m.confirming = true
	m.confirmYes = true

	view := m.ConfirmationView()
	if !strings.Contains(view, "Are you sure you wanted to remove Go ?") {
		t.Errorf("confirmation missing prompt, got %q", view)
	}
	if !strings.Contains(view, "Yes") {
		t.Errorf("confirmation missing [Yes] control, got %q", view)
	}
	if !strings.Contains(view, "No") {
		t.Errorf("confirmation missing [No] control, got %q", view)
	}

	upd, _ := m.Update(UninstalledMsg{Name: Go})
	m2 := upd.(InstalledModel)
	if m2.Confirming() {
		t.Errorf("expected confirming false after uninstall")
	}
	if hint := CurrentFooterHint(); hint.Text == "" {
		t.Errorf("expected a footer hint after uninstall")
	}
	ClearFooterHint()
}
