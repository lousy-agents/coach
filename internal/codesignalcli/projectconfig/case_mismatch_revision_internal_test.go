package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// The check is driven directly because a file system that tells cases apart stops requireOnDisk at a
// missing parent before any spelling is compared; the lookup of the revision is the real one.
func TestTheSpellingCheckAdvisesFromTheRevisionForADirectorySpelledInAnotherCase(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, repo string)
		typed string
		want  []string
		avoid []string
	}{
		{"the revision holds the disk's spelling", func(t *testing.T, repo string) {
			gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
		}, "CFG/p.json", []string{`it holds "cfg"; pass --project-config cfg/p.json`}, []string{"rename"}},
		{"the revision holds the typed spelling and the disk was renamed", func(t *testing.T, repo string) {
			gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
			if err := os.Rename(filepath.Join(repo, "cfg"), filepath.Join(repo, "CFG")); err != nil {
				t.Fatal(err)
			}
		}, "cfg/p.json", []string{`it holds "cfg"; rename "CFG" back to "cfg", or commit the rename and pass --project-config CFG/p.json`}, nil},
		{"the revision holds neither spelling", func(t *testing.T, repo string) {
			gitfixture.CommitFile(t, repo, "go.mod", "module m\n")
			if err := os.MkdirAll(filepath.Join(repo, "CFG"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, "CFG", "p.json"), []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "cfg/p.json", []string{`it holds neither "cfg" nor "CFG"`}, []string{"rename", "pass --project-config"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := gitfixture.Init(t)
			c.setup(t, repo)

			assertSpellingAdviceFromRevision(t, repo, c.typed, c.want, c.avoid)
		})
	}
}

func assertSpellingAdviceFromRevision(t *testing.T, repo, typed string, want, avoid []string) {
	t.Helper()
	root, err := os.OpenRoot(repo)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()

	advised := adviseOnRevision(checkExactNames(root, typed), typed, entriesThrough(runProjectConfigGit, repo, "HEAD"))

	var found *ConfigError
	if !errors.As(advised, &found) {
		t.Fatalf("advised = %v; want a *ConfigError", advised)
	}
	assertCaseMismatchAdvice(t, found, want, avoid)
}
