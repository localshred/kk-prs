package board

import "testing"

func TestVersionLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"release-9", "release-10", true},
		{"release-10", "release-9", false},
		{"release-1.9", "release-1.10", true},
		{"release-A", "release-B", true},
		{"release/2.0", "release/10.0", true},
		{"release-A", "release-A", false},
	}
	for _, tt := range tests {
		if got := VersionLess(tt.a, tt.b); got != tt.want {
			t.Errorf("VersionLess(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
