package codesignalcli

import (
	"reflect"
	"testing"
)

var hiddenRootsTracked = []string{"README.md", "go.mod", "vendor/a/x.go", "vendor/b/y.go", "mixed/hidden.go", "mixed/visible.go"}

func TestHiddenRootsNameTheDirectoryThatHoldsWhatGitHides(t *testing.T) {
	tests := []struct {
		name   string
		hidden []string
		want   []string
	}{
		{"a hidden subtree collapses to its top directory, a mixed directory names itself", []string{"vendor/a/x.go", "vendor/b/y.go", "mixed/hidden.go"}, []string{"mixed", "vendor"}},
		{"half a subtree stays at the directory that is fully hidden", []string{"vendor/a/x.go"}, []string{"vendor/a"}},
		{"a hidden file at the repository root names the root", []string{"go.mod"}, []string{"."}},
		{"every tracked file hidden collapses to the root", hiddenRootsTracked, []string{"."}},
		{"nothing hidden names nothing", nil, nil},
	}
	for _, tt := range tests {
		assertHiddenRoots(t, tt.name, tt.hidden, tt.want)
	}
}

func assertHiddenRoots(t *testing.T, name string, hidden, want []string) {
	t.Helper()
	if got := hiddenRoots(hidden, hiddenRootsTracked); !reflect.DeepEqual(got, want) {
		t.Errorf("%s: hiddenRoots(%q) = %q, want %q", name, hidden, got, want)
	}
}
