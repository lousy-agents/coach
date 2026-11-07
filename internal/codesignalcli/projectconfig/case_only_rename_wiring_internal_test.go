package projectconfig

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// The rename is made through a temporary name, which works on a file system that folds case and on one that does not,
// and core.ignorecase is set by hand. The move is printed where the name on disk is the committed file seen under
// another case, which is what a disk that folds case makes of the rename; on a disk that tells cases apart, a hard link
// from the committed name to the new one stands in for that identity, so the wiring runs on every machine.
func TestAnUncommittedPolicyRenamedInCaseOnlyIsToldHowToRecordTheRename(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "policies/p.json", "{}")
	if output, err := exec.Command("git", "-C", repo, "config", "core.ignorecase", "true").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v: %s", err, output)
	}
	from, to := filepath.Join(repo, "policies", "p.json"), filepath.Join(repo, "policies", "P.json")
	if err := os.Rename(from, from+".tmp"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(from+".tmp", to); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(from); errors.Is(err, fs.ErrNotExist) {
		if err := os.Link(to, from); err != nil {
			t.Fatal(err)
		}
	}

	_, err := LoadExplicit(repo, "HEAD", "policies/P.json")

	var found *ConfigError
	if !errors.As(err, &found) {
		t.Fatalf("LoadExplicit error = %v; want a *ConfigError", err)
	}
	for _, want := range []string{`the analyzed revision holds the path as "policies/p.json"`, gitAtTheRoot + " mv -- 'policies/p.json' 'policies/P.json'"} {
		if !strings.Contains(found.Message, want) {
			t.Errorf("message %q; want it to contain %q", found.Message, want)
		}
	}
	if strings.Contains(found.Message, "commit the file so it is readable") {
		t.Errorf("message %q still tells the user to commit a file that git add cannot record", found.Message)
	}
}
