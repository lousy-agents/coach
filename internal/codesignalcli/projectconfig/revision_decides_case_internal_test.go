package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// The analyzed revision decides what a policy path names. When it holds exactly the path the user typed, that
// is the policy that is read, whatever spelling a file system that folds case shows for the same file: the
// disk is no more than where the file is looked for, and `git status` is clean when only the case differs.
// v0.11.0 read it. A refusal is for a revision that does not hold the typed spelling.
func TestAPolicyTheRevisionHoldsExactlyAsTypedIsReadWhateverSpellingTheDiskShows(t *testing.T) {
	cases := []struct {
		name     string
		disk     foldingDisk
		revision fakeRevision
		path     string
	}{
		{"the file is spelled in another case on disk", foldingDiskHolding("cfg/P.json"), fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}, "cfg/p.json"},
		{"the directory is spelled in another case on disk", foldingDiskHolding("CFG/p.json"), fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}, "cfg/p.json"},
		{"the directory and the file both are", foldingDiskHolding("CFG/P.JSON"), fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}, "cfg/p.json"},
		{"the revision holds the spelling of the disk as well", foldingDiskHolding("CFG/p.json"), fakeRevision{"": {"cfg", "CFG"}, "cfg": {"p.json"}, "CFG": {"p.json"}}, "cfg/p.json"},
		{"a policy moved to the path Coach reads, which the directory the move left behind spells otherwise", foldingDiskHolding(".Lousy-Agents/coach.json"), fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"coach.json"}}, ".lousy-agents/coach.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertAcceptedAsTyped(t, c.disk, c.revision, c.path)
		})
	}
}

func assertAcceptedAsTyped(t *testing.T, disk foldingDisk, revision fakeRevision, repoPath string) {
	t.Helper()

	err := adviseOnRevision(checkRegularFileThrough(disk, repoPath), repoPath, revision.entries)

	if err != nil {
		t.Errorf("a policy the revision holds exactly as typed was refused: %v", err)
	}
}

// What the disk lacks is still what it lacks. With the file missing from a directory that the disk spells in
// another case, the revision holding the typed path means the spelling is no reason to refuse, and the
// missing file is reported as missing, so that the restore that follows writes it where the disk resolves it.
func TestAMissingPolicyUnderADirectorySpelledOtherwiseOnDiskIsMissingNotMisspelled(t *testing.T) {
	disk := foldingDiskHolding("CFG/other.json")
	revision := fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}

	found := configErrorOf(t, adviseOnRevision(checkRegularFileThrough(disk, "cfg/p.json"), "cfg/p.json", revision.entries))

	if found.Kind != KindMissing || found.PolicyCause == projectreadiness.CauseNameCaseDiffers {
		t.Errorf("got kind %v, cause %q, %q; want the missing file, not the spelling of its directory", found.Kind, found.PolicyCause, found.Message)
	}
}

// A revision that does not hold the typed path is still refused, with the way out its own spelling gives.
func TestAPolicyTheRevisionDoesNotHoldAsTypedIsStillRefusedForItsSpelling(t *testing.T) {
	cases := []struct {
		name     string
		disk     foldingDisk
		revision revisionEntries
		path     string
		want     string
	}{
		{"it holds the spelling of the disk", foldingDiskHolding("cfg/P.json"), fakeRevision{"": {"cfg"}, "cfg": {"P.json"}}.entries, "cfg/p.json", `it holds "cfg/P.json"; pass --project-config cfg/P.json`},
		{"it holds neither", foldingDiskHolding("cfg/P.json"), fakeRevision{"": {"cfg"}, "cfg": {"other.json"}}.entries, "cfg/p.json", `it holds neither "cfg/p.json" nor "cfg/P.json"`},
		{"it cannot be asked", foldingDiskHolding("cfg/P.json"), failingRevision, "cfg/p.json", `is stored as "P.json" on disk, not "p.json"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRefusedForItsSpelling(t, c.disk, c.revision, c.path, c.want)
		})
	}
}

func assertRefusedForItsSpelling(t *testing.T, disk foldingDisk, revision revisionEntries, repoPath, want string) {
	t.Helper()

	err := adviseOnRevision(checkRegularFileThrough(disk, repoPath), repoPath, revision)

	assertCaseMismatchAdvice(t, configErrorOf(t, err), []string{want}, nil)
}

func TestHoldsPathNeedsEveryComponentSpelledAsGiven(t *testing.T) {
	revision := fakeRevision{"": {"a", "Z"}, "a": {"b"}, "a/b": {"c.json"}, "Z": {"d.json"}}
	cases := []struct {
		path string
		want bool
	}{
		{"a/b/c.json", true},
		{"a", true},
		{"a/b/C.json", false},
		{"A/b/c.json", false},
		{"a/x/c.json", false},
		{"a/b/c.json/deeper", false},
		{"z/d.json", false},
	}
	for _, c := range cases {
		if got, err := revisionEntries(revision.entries).holdsPath(c.path); err != nil || got != c.want {
			t.Errorf("holdsPath(%q) = %v, %v; want %v", c.path, got, err, c.want)
		}
	}
}

func TestHoldsPathReportsAListingThatFailed(t *testing.T) {
	if got, err := revisionEntries(failingRevision).holdsPath("a/b.json"); err == nil || got {
		t.Errorf("holdsPath = %v, %v; want the failed listing, not a verdict", got, err)
	}
}
