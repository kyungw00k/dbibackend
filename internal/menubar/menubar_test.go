package menubar

import (
	"reflect"
	"testing"
)

func TestAppendUniquePath(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		dir      string
		want     []string
	}{
		// Regression: osascript returns trailing-slash paths while shell
		// args don't — both used to be appended, doubling the file list
		// on the Switch.
		{"trailing slash duplicate", []string{"/a/switch/"}, "/a/switch", []string{"/a/switch/"}},
		{"exact duplicate", []string{"/a"}, "/a", []string{"/a"}},
		{"clean append", []string{"/a"}, "/b", []string{"/a", "/b"}},
		{"child overlap rejected", []string{"/a"}, "/a/sub", []string{"/a"}},
		{"parent overlap rejected", []string{"/a/sub"}, "/a", []string{"/a/sub"}},
		{"empty list", nil, "/a", []string{"/a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := appendUniquePath(tt.existing, tt.dir); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("appendUniquePath(%v, %q) = %v, want %v", tt.existing, tt.dir, got, tt.want)
			}
		})
	}
}

func TestNormalizePaths(t *testing.T) {
	// stored paths are re-saved in cleaned form, collapsing the
	// trailing-slash variant into one entry
	got := normalizePaths([]string{"/a/switch/", "/a/switch", "/b"})
	want := []string{"/a/switch", "/b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizePaths() = %v, want %v", got, want)
	}
}
