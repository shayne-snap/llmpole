package cli

import "testing"

func TestLooksLikeRepoID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"valid org/repo", "org/repo", true},
		{"valid short", "a/b", true},
		{"empty", "", false},
		{"blank", "  \t  ", false},
		{"single segment", "only", false},
		{"three segments", "a/b/c", false},
		{"has space", "org/repo name", false},
		{"trailing newline trimmed", "org/repo\n", true},
		{"org empty", "/repo", false},
		{"repo empty", "org/", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeRepoID(tt.in)
			if got != tt.want {
				t.Errorf("looksLikeRepoID(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
