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

func TestCheckExactNamesRefusesASpellingThatDiffersOnlyByCase(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  string
	}{
		{"the file", []string{"cfg/P.JSON"}, `"P.JSON"`},
		{"a parent directory", []string{"CFG/p.json"}, `"CFG"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkExactNames(openRootWith(t, c.files...), "cfg/p.json")

			var found *ConfigError
			if !errors.As(err, &found) || found.Kind != KindInvalid {
				t.Fatalf("checkExactNames = %v, want an invalid-policy ConfigError", err)
			}
			if !strings.Contains(found.Message, c.want) || !strings.Contains(found.Message, "project_config_invalid") {
				t.Errorf("message = %q, want it to name %s and carry project_config_invalid", found.Message, c.want)
			}
		})
	}
}

func TestCheckExactNamesAcceptsTheExactSpelling(t *testing.T) {
	if err := checkExactNames(openRootWith(t, "cfg/p.json"), "cfg/p.json"); err != nil {
		t.Errorf("checkExactNames = %v, want nil", err)
	}
}

func TestCheckExactNamesPrefersTheExactSpellingOverACaseVariant(t *testing.T) {
	root := openRootWith(t, "cfg/p.json", "cfg/P.JSON")
	listing, err := root.Open("cfg")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listing.Close() }()
	names, err := listing.Readdirnames(-1)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Skip("this filesystem folds case, so both spellings cannot exist")
	}

	if err := checkExactNames(root, "cfg/p.json"); err != nil {
		t.Errorf("checkExactNames = %v, want nil when the exact spelling is among the entries", err)
	}
}

func TestCheckExactNamesLeavesAnUnlistedNameToTheOtherChecks(t *testing.T) {
	if err := checkExactNames(openRootWith(t, "cfg/other.json"), "cfg/p.json"); err != nil {
		t.Errorf("checkExactNames = %v, want nil: a name nothing lists is not a case difference", err)
	}
}
