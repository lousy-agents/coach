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
		placeOnDisk(t, root, name)
	}
	if _, err := os.Lstat(filepath.Join(root, "CASE-PROBE-NOT-THERE")); err == nil {
		t.Skip("needs a file system that tells cases apart")
	}
	return root
}

// placeOnDisk makes name under root: a directory for a name ending in "/", a file otherwise.
func placeOnDisk(t *testing.T, root, name string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(name, "/")))
	if strings.HasSuffix(name, "/") {
		requireNoError(t, os.MkdirAll(target, 0o755))
		return
	}
	requireNoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	requireNoError(t, os.WriteFile(target, []byte("{}"), 0o644))
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func untracked(string) bool { return false }

// The default policy's path named in another case is never told to be renamed on disk to the spelling typed, which a
// scan without --project-config does not read.
func TestTheDefaultPolicyNamedInAnotherCaseIsToldToRenameToTheDefaultSpelling(t *testing.T) {
	refusal := requireCaseRefusal(t, wrongCaseUnder(diskWith(t, ".Lousy-Agents/coach.json"), ".lousy-agents/COACH.json", chosenByTheUser, untracked))

	requireContains(t, refusal.Message, `rename the file on disk to ".lousy-agents/coach.json"`)
	requireLacks(t, refusal.Message, `to ".lousy-agents/COACH.json"`)
}

// The default spelling beside another is the one to keep, never the one to remove.
func TestTheDefaultPolicyNamedInAnotherCaseKeepsTheDefaultSpellingAmongSeveral(t *testing.T) {
	refusal := requireCaseRefusal(t, wrongCaseUnder(diskWith(t, ".lousy-agents/coach.json", ".lousy-agents/Coach.json"), ".lousy-agents/COACH.json", chosenByTheUser, untracked))

	requireContains(t, refusal.Message, `To keep ".lousy-agents/coach.json"`)
	requireLacks(t, refusal.Message, `remove ".lousy-agents/coach.json"`)
}

func requireCaseRefusal(t *testing.T, refusal *ConfigError) *ConfigError {
	t.Helper()
	if refusal == nil {
		t.Fatal("wrongCaseUnder = nil; want the case refusal")
	}
	return refusal
}

func requireLacks(t *testing.T, message, unwanted string) {
	t.Helper()
	if strings.Contains(message, unwanted) {
		t.Errorf("message %q contains %q", message, unwanted)
	}
}

// A case variant that is not a regular file is no policy under another name: it is not refused for its case.
func TestACaseVariantThatIsNotARegularFileIsNoCaseRefusal(t *testing.T) {
	if refusal := wrongCaseUnder(diskWith(t, ".lousy-agents/Coach.json/"), CanonicalPath, chosenByCoach, untracked); refusal != nil {
		t.Errorf("wrongCaseUnder = %q; want nil for a directory in another case", refusal.Message)
	}
}

// A policy Coach finds is read at another spelling only with --project-config, so the advice says so for several
// spellings as for one.
func TestSeveralSpellingsOfThePolicyCoachFindsNameTheFlag(t *testing.T) {
	refusal := requireCaseRefusal(t, wrongCaseUnder(diskWith(t, ".Lousy-Agents/coach.json", ".LOUSY-AGENTS/coach.json"), CanonicalPath, chosenByCoach, untracked))

	requireContains(t, refusal.Message, "with --project-config")
}
