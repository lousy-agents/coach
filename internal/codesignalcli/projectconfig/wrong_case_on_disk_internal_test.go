package projectconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// diskWith is a directory holding the given files, or directories for names ending in "/".
func diskWith(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range names {
		target := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(name, "/")))
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Lstat(filepath.Join(root, "CASE-PROBE-NOT-THERE")); err == nil {
		t.Skip("needs a file system that tells cases apart")
	}
	return root
}

// The default policy's path named in another case is never told to be renamed on disk to the spelling typed, which a
// scan without --project-config does not read, and the default spelling is never the one to remove.
func TestTheDefaultPolicyNamedInAnotherCaseIsToldToKeepTheDefaultSpelling(t *testing.T) {
	t.Run("one other spelling that is not the default", func(t *testing.T) {
		refusal := wrongCaseUnder(diskWith(t, ".Lousy-Agents/coach.json"), ".lousy-agents/COACH.json", chosenByTheUser)

		if refusal == nil {
			t.Fatal("wrongCaseUnder = nil; want the case refusal")
		}
		requireContains(t, refusal.Message, `rename the file on disk to ".lousy-agents/coach.json"`)
		if strings.Contains(refusal.Message, `to ".lousy-agents/COACH.json"`) {
			t.Errorf("message %q tells the user to rename to the spelling typed", refusal.Message)
		}
	})
	t.Run("the default spelling beside another", func(t *testing.T) {
		refusal := wrongCaseUnder(diskWith(t, ".lousy-agents/coach.json", ".lousy-agents/Coach.json"), ".lousy-agents/COACH.json", chosenByTheUser)

		if refusal == nil {
			t.Fatal("wrongCaseUnder = nil; want the case refusal")
		}
		requireContains(t, refusal.Message, `To keep ".lousy-agents/coach.json"`)
		if strings.Contains(refusal.Message, `remove ".lousy-agents/coach.json"`) {
			t.Errorf("message %q tells the user to remove the default spelling", refusal.Message)
		}
	})
}

// A case variant that is not a regular file is no policy under another name: it is not refused for its case.
func TestACaseVariantThatIsNotARegularFileIsNoCaseRefusal(t *testing.T) {
	if refusal := wrongCaseUnder(diskWith(t, ".lousy-agents/Coach.json/"), CanonicalPath, chosenByCoach); refusal != nil {
		t.Errorf("wrongCaseUnder = %q; want nil for a directory in another case", refusal.Message)
	}
}

// A policy Coach finds is read at another spelling only with --project-config, so the advice says so for several
// spellings as for one.
func TestSeveralSpellingsOfThePolicyCoachFindsNameTheFlag(t *testing.T) {
	refusal := wrongCaseUnder(diskWith(t, ".Lousy-Agents/coach.json", ".LOUSY-AGENTS/coach.json"), CanonicalPath, chosenByCoach)

	if refusal == nil {
		t.Fatal("wrongCaseUnder = nil; want the case refusal")
	}
	requireContains(t, refusal.Message, "with --project-config")
}
