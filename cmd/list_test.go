package cmd

import "testing"

func TestShortProject(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/Users/alice/projects/myapp", "projects/myapp"},
		{"short", "short"},
		{"a/b", "a/b"},
		{"/a", "/a"},
		{"one/two/three/four", "three/four"},
		{"", ""},
	}

	for _, tt := range tests {
		got := shortProject(tt.input)
		if got != tt.want {
			t.Errorf("shortProject(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
