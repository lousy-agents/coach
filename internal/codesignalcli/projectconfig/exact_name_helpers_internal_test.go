package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openRootWith(t *testing.T, files ...string) *os.Root {
	t.Helper()
	dir := t.TempDir()
	for _, name := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}

func assertCaseDifferenceRefused(t *testing.T, files []string, want string) {
	t.Helper()
	checked := checkExactNames(openRootWith(t, files...), "cfg/p.json")
	var mismatch *caseMismatchError
	if !errors.As(checked, &mismatch) {
		t.Fatalf("checkExactNames = %v, want a case mismatch for the revision to be asked about", checked)
	}
	err := adviseOnRevision(checked, "cfg/p.json", nil)

	var found *ConfigError
	if !errors.As(err, &found) || found.Kind != KindInvalid {
		t.Fatalf("checkExactNames = %v, want an invalid-policy ConfigError", err)
	}
	if !strings.Contains(found.Message, want) || !strings.Contains(found.Message, "project_config_invalid") {
		t.Errorf("message = %q, want it to name %s and carry project_config_invalid", found.Message, want)
	}
}
