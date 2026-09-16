package api

import (
	"encoding/json"
	"testing"
)

func TestCompareVersionsNonEmpty(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"3.14.7", "3.14.6", 1},
		{"3.14.7", "3.14.7", 0},
		{"3.14.7", "3.14.10", -1},
		{"3.14.7", "3.13.99", 1},
		{"3.11.0", "3.10.99", 1},
		{"20.1.0", "3.14.7", 1},
	}
	for _, c := range cases {
		if got := compareVersionsNonEmpty(c.a, c.b); got != c.want {
			t.Errorf("compareVersionsNonEmpty(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestPepsStageParsing(t *testing.T) {
	raw := `{
		"metadata": {},
		"releases": {
		"3.14": [
			{"stage": "3.14.7", "state": "actual", "date": "2025-10-07", "note": ""},
			{"stage": "3.14.0 candidate 1", "state": "actual", "date": "2024-12-05", "note": ""},
			{"stage": "3.14.0", "state": "expected", "date": "2024-09-27", "note": ""}
		],
		"3.13": [
			{"stage": "3.13.12", "state": "actual", "date": "2025-09-02", "note": ""},
			{"stage": "3.13.0a1", "state": "actual", "date": "2023-10-02", "note": ""}
		],
		"3.11": [
			{"stage": "3.11.8", "state": "actual", "date": "2024-02-06", "note": ""},
			{"stage": "3.11.0 final", "state": "actual", "date": "2022-10-24", "note": ""}
		],
		"3.10": [
			{"stage": "3.10.20", "state": "actual", "date": "2026-03-03", "note": ""}
		]
		}
	}`

	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	var series map[string]json.RawMessage
	if err := json.Unmarshal(doc["releases"], &series); err != nil {
		t.Fatalf("unmarshal series: %v", err)
	}

	versions := pythonReleaseInfosFromSeries(series)

	want := []PythonReleaseInfo{
		{Version: "3.14.7", Date: "2025-10-07"},
		{Version: "3.13.12", Date: "2025-09-02"},
		{Version: "3.11.8", Date: "2024-02-06"},
		{Version: "3.11.0", Date: "2022-10-24"},
	}
	if len(versions) != len(want) {
		t.Fatalf("versions = %v, want %v", versions, want)
	}
	for i, w := range want {
		if versions[i] != w {
			t.Errorf("versions[%d] = %+v, want %+v", i, versions[i], w)
		}
	}
}

func TestPythonSeriesSupported(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"3.14.7", true},
		{"3.13.12", true},
		{"3.11.0", true},
		{"3.10.20", false},
		{"3.9.25", false},
		{"3.8.20", false},
		{"2.7.18", false},
		{"notaversion", false},
	}
	for _, c := range cases {
		if got := pythonSeriesSupported(c.version); got != c.want {
			t.Errorf("pythonSeriesSupported(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}

func TestPythonArtifactForWindowsAMD64FromManifest(t *testing.T) {
	fixture := pythonWindowsManifest{
		Versions: []pythonWindowsPackage{
			{
				ID:  "pythoncore-3.14-32",
				URL: "https://www.python.org/ftp/python/3.14.7/python-3.14.7-win32.zip",
			},
			{
				ID:  "pythoncore-3.14-64",
				URL: "https://www.python.org/ftp/python/3.14.7/python-3.14.7-amd64.zip",
				Hash: pythonPackageHash{
					SHA256: "abc123def456",
				},
			},
			{
				ID:  "pythoncore-3.14t-64",
				URL: "https://www.python.org/ftp/python/3.14.7/python-3.14.7t-amd64.zip",
			},
			{
				ID:  "pythonembed-3.14-64",
				URL: "https://www.python.org/ftp/python/3.14.7/python-3.14.7-embed-amd64.zip",
			},
			{
				ID:  "pythontest-3.14-64",
				URL: "https://www.python.org/ftp/python/3.14.7/python-3.14.7-test-amd64.zip",
			},
			{
				ID:  "pythoncore-3.14-64",
				URL: "https://www.python.org/ftp/python/3.14.7/python-3.14.7-arm64.zip",
				Hash: pythonPackageHash{
					SHA256: "789xyz",
				},
			},
		},
	}

	body, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var manifest pythonWindowsManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}

	amd64, ok := matchWindowsManifest(&manifest, "3.14.7", "amd64")
	if !ok {
		t.Fatal("expected match for amd64 full package")
	}
	if want := "python-3.14.7-amd64.zip"; amd64.Filename != want {
		t.Errorf("filename = %q, want %q", amd64.Filename, want)
	}
	if amd64.SHA256 != "abc123def456" {
		t.Errorf("amd64 sha256 = %q, want %q", amd64.SHA256, "abc123def456")
	}
	if amd64.OS != "windows" || amd64.Arch != "amd64" {
		t.Errorf("got %s/%s, want windows/amd64", amd64.OS, amd64.Arch)
	}
	if amd64.Kind != "archive" {
		t.Errorf("kind = %q, want archive", amd64.Kind)
	}

	arm64, ok := matchWindowsManifest(&manifest, "3.14.7", "arm64")
	if !ok {
		t.Fatal("expected match for arm64 full package")
	}
	if want := "python-3.14.7-arm64.zip"; arm64.Filename != want {
		t.Errorf("filename = %q, want %q", arm64.Filename, want)
	}
	if arm64.SHA256 != "789xyz" {
		t.Errorf("arm64 sha256 = %q, want %q", arm64.SHA256, "789xyz")
	}

	if _, ok := matchWindowsManifest(&manifest, "3.14.7", "386"); ok {
		t.Error("expected no match for unsupported arch token")
	}
}

func TestPythonArtifactForUnsupportedOSReturnsFalse(t *testing.T) {
	for _, os := range []string{"darwin", "linux"} {
		if _, ok := pythonArtifactForMachine("3.14.7", os, "arm64"); ok {
			t.Errorf("pythonArtifactForMachine should return false for %s", os)
		}
	}
}

func TestFetchPythonVersionsFromFTPIndex(t *testing.T) {
	fixture := `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">
<html><head><title>Index of /ftp/python</title></head><body>
<h1>Index of /ftp/python</h1>
<pre><img src="/icons/folder.gif" alt="[DIR]"> <a href="../">Parent Directory</a>
<img src="/icons/folder.gif" alt="[DIR]"> <a href="3.14.7/">3.14.7/</a>              2025-10-07 15:30    -
<img src="/icons/folder.gif" alt="[DIR]"> <a href="3.13.9/">3.13.9/</a>              2025-10-14 17:43    -
<img src="/icons/folder.gif" alt="[DIR]"> <a href="3.12.10/">3.12.10/</a>             2025-04-08 12:00    -
<img src="/icons/folder.gif" alt="[DIR]"> <a href="3.10.11/">3.10.11/</a>             2023-04-05 12:00    -
<img src="/icons/folder.gif" alt="[DIR]"> <a href="2.7.18/">2.7.18/</a>              2020-04-20 12:00    -
<img src="/icons/compressed.gif" alt="   "> <a href="python-3.14.7-amd64.exe">python-3.14.7-amd64.exe</a>
</pre></body></html>`

	var found []string
	for _, m := range pythonHrefRe.FindAllStringSubmatch(fixture, -1) {
		if sub := pythonFTPDirRe.FindStringSubmatch(m[1]); sub != nil {
			found = append(found, sub[1])
		}
	}
	if len(found) != 5 {
		t.Fatalf("expected 5 version dirs, got %d: %v", len(found), found)
	}
	for _, v := range found {
		if v == "python-3.14.7-amd64.exe" {
			t.Error("file href must not be matched as a version directory")
		}
	}
}
