package process

import "testing"

func TestExtractPythonVersion(t *testing.T) {
	cases := []struct {
		output, want string
	}{
		{"Python 3.14.7", "3.14.7"},
		{"Python 3.11.0", "3.11.0"},
		{"unknown", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := extractPythonVersion(c.output); got != c.want {
			t.Errorf("extractPythonVersion(%q) = %q, want %q", c.output, got, c.want)
		}
	}
}

func TestExtractClangVersion(t *testing.T) {
	for _, out := range []string{"clang version 20.1.8", "Ubuntu clang version 16.0.6"} {
		if got := extractClangVersion(out); got == "" {
			t.Errorf("extractClangVersion(%q) returned empty", out)
		}
	}
}
