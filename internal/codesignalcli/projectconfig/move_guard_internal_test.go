package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func aWorkingTreeHolding(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, contents := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// A move that changes only the case of a name is printed where nothing stands at the destination, or where what stands
// there is the committed file itself. Anything else is a different file, which git mv would delete when git is set to
// ignore case on a disk that does not.
func TestAMoveIsPrintedOnlyWhereNothingElseStandsAtTheDestination(t *testing.T) {
	root := aWorkingTreeHolding(t, map[string]string{
		"dir/Committed.json": "committed",
		"dir/other.json":     "the user's",
		"plain":              "a file",
	})
	if err := os.Link(filepath.Join(root, "dir/Committed.json"), filepath.Join(root, "dir/linked.json")); err != nil {
		t.Skipf("this disk cannot make a hard link: %v", err)
	}
	cases := []struct {
		name     string
		from, to string
		want     bool
	}{
		{"the destination is absent", "dir/Committed.json", "dir/new.json", true},
		{"the destination is under a directory that is absent", "dir/Committed.json", "elsewhere/new.json", true},
		{"the destination is the committed file itself, under another name", "dir/Committed.json", "dir/linked.json", true},
		{"a different file stands at the destination", "dir/Committed.json", "dir/other.json", false},
		{"the committed file is gone and something stands at the destination", "dir/Gone.json", "dir/other.json", false},
		{"the destination is under a file", "dir/Committed.json", "plain/new.json", false},
		{"a directory stands at the destination", "dir/Committed.json", "dir", false},
	}
	for _, c := range cases {
		assertMoveGuard(t, root, c.name, c.from, c.to, c.want)
	}
}

func assertMoveGuard(t *testing.T, root, name, from, to string, want bool) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		if got := nothingElseStandsAt(root, from, to); got != want {
			t.Errorf("nothingElseStandsAt(%q, %q) = %v; want %v", from, to, got, want)
		}
	})
}

func TestTheGuardAllowsNoMoveWhenTheRepositoryRootCannotBeFound(t *testing.T) {
	findsNoRoot := func(string, ...string) ([]byte, error) {
		return nil, errors.New("exit status 128: fatal: not a git repository")
	}

	if moveGuardFor(findsNoRoot, "repo")("a/Committed.json", "a/new.json") {
		t.Errorf("a move was allowed with no root to look under")
	}
}

func TestTheGuardLooksUnderTheRepositoryRoot(t *testing.T) {
	root := aWorkingTreeHolding(t, map[string]string{"a/Committed.json": "c", "a/taken.json": "u"})
	answersRoot := func(string, ...string) ([]byte, error) { return []byte(root + "\n"), nil }
	guard := moveGuardFor(answersRoot, "repo")

	if !guard("a/Committed.json", "a/free.json") || guard("a/Committed.json", "a/taken.json") {
		t.Errorf("the guard did not tell the free destination from the taken one")
	}
}

// Where a different file stands at the destination, each place that prints a move prints none and says why; where
// nothing does, each prints it.
func TestEveryAdviceThatPrintsAMoveHoldsItBehindTheGuard(t *testing.T) {
	deny := func(string, string) bool { return false }
	typed, held := []string{"policies", "P.json"}, []string{"policies", "p.json"}

	for name, advice := range map[string]struct{ allowed, denied string }{
		"named policy":      {renameAdvice(typed, held, anyMove), renameAdvice(typed, held, deny)},
		"discovered policy": {moveAdvice(".lousy-agents/Coach.json", ".lousy-agents/coach.json", anyMove), moveAdvice(".lousy-agents/Coach.json", ".lousy-agents/coach.json", deny)},
		"several spellings": {
			severalSpellingsReason([]string{".lousy-agents/Coach.json", ".lousy-agents/COACH.json"}, false, ".lousy-agents/coach.json", anyMove),
			severalSpellingsReason([]string{".lousy-agents/Coach.json", ".lousy-agents/COACH.json"}, false, ".lousy-agents/coach.json", deny),
		},
	} {
		assertMoveHeldBehindTheGuard(t, name, advice.allowed, advice.denied)
	}
}

func assertMoveHeldBehindTheGuard(t *testing.T, name, allowed, denied string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		if !strings.Contains(allowed, "mv -- ") {
			t.Errorf("advice %q; want the move where it is allowed", allowed)
		}
		if strings.Contains(denied, "mv -- ") || strings.Contains(denied, "rm --cached") || !strings.Contains(denied, "a different file stands at") {
			t.Errorf("advice %q; want no command and the reason where a different file stands at the destination", denied)
		}
	})
}
