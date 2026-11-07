package projectconfig

import (
	"errors"
	"testing"
)

// These run on every file system because the disk is injected. The specs that use a real one skip where
// the disk tells cases apart, which is every CI machine, so without these the calls that refuse a name
// spelled in another case could be deleted and nothing would fail.
func TestTheOnDiskCheckRefusesAPathTheDiskSpellsInAnotherCase(t *testing.T) {
	cases := []struct {
		name       string
		stored     []string
		typed      string
		wantTyped  string
		wantOnDisk string
	}{
		{"a file", []string{"cfg/P.json"}, "cfg/p.json", "p.json", "P.json"},
		{"a directory", []string{"CFG/p.json"}, "cfg/p.json", "cfg", "CFG"},
		{"a directory above a file spelled in another case too", []string{"CFG/P.json"}, "cfg/p.json", "cfg", "CFG"},
		{"a directory above a file that is not there", []string{"CFG/other.json"}, "cfg/p.json", "cfg", "CFG"},
		{"a file at the root", []string{"P.json"}, "p.json", "p.json", "P.json"},
		{"a directory above a directory that is not there either", []string{"cfg/p.json"}, "Cfg/new/zz.json", "Cfg", "cfg"},
		{"the same at the first level of a source tree", []string{"src/a.go"}, "SRC/new/p.json", "SRC", "src"},
		{"a directory above two that are not there", []string{"cfg/p.json"}, "CFG/a/b/zz.json", "CFG", "cfg"},
		{"the second directory, below one spelled as typed, with a third not there", []string{"cfg/sub/x.json"}, "cfg/SUB/new/p.json", "SUB", "sub"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertCaseMismatch(t, checkRegularFileThrough(foldingDiskHolding(c.stored...), c.typed), c.wantTyped, c.wantOnDisk)
		})
	}
}

func assertCaseMismatch(t *testing.T, err error, wantTyped, wantOnDisk string) {
	t.Helper()
	var mismatch caseMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("checkRegularFileThrough = %v; want a case mismatch", err)
	}
	if mismatch.typed != wantTyped || mismatch.onDisk != wantOnDisk {
		t.Errorf("mismatch = typed %q, on disk %q; want typed %q, on disk %q", mismatch.typed, mismatch.onDisk, wantTyped, wantOnDisk)
	}
}

func TestTheOnDiskCheckAcceptsOrCallsMissingWhatTheDiskSpellsAsTyped(t *testing.T) {
	cases := []struct {
		name        string
		stored      []string
		wantMissing bool
	}{
		{"the file as typed", []string{"cfg/p.json"}, false},
		{"no file, and the directory as typed", []string{"cfg/other.json"}, true},
		{"no file, and no such directory", []string{"docs/x.json"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertAcceptedOrMissing(t, checkRegularFileThrough(foldingDiskHolding(c.stored...), "cfg/p.json"), c.wantMissing)
		})
	}
}

func assertAcceptedOrMissing(t *testing.T, err error, wantMissing bool) {
	t.Helper()
	if !wantMissing {
		if err != nil {
			t.Errorf("checkRegularFileThrough = %v; want nil", err)
		}
		return
	}
	if found := configErrorOf(t, err); found.Kind != KindMissing {
		t.Errorf("error = kind %v, %q; want a missing policy", found.Kind, found.Message)
	}
}

func TestAMismatchOnAFoldingDiskIsAdvisedFromTheRevision(t *testing.T) {
	disk := foldingDiskHolding("cfg/policy.json")
	revision := fakeRevision{"": {"cfg"}, "cfg": {"policy.json"}}.entries

	advised := adviseOnRevision(checkRegularFileThrough(disk, "cfg/POLICY.json"), "cfg/POLICY.json", revision)

	assertCaseMismatchAdvice(t, configErrorOf(t, advised), []string{`it holds "cfg/policy.json"; pass --project-config cfg/policy.json`}, []string{"rename"})
}

// A missing deeper directory does not excuse a parent that exists in another case: the policy would be
// authored where the file system resolves that parent, in a directory the scan then refuses. Where every
// directory that exists is spelled as typed, the policy is just missing.
func TestAMissingDirectoryBelowParentsSpelledAsTypedLeavesThePolicyMissing(t *testing.T) {
	cases := []struct {
		name   string
		stored []string
		typed  string
	}{
		{"one directory missing", []string{"cfg/p.json"}, "cfg/new/zz.json"},
		{"two directories missing", []string{"cfg/p.json"}, "cfg/a/b/zz.json"},
		{"the first directory missing", []string{"docs/x.json"}, "cfg/new/zz.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertAcceptedOrMissing(t, checkRegularFileThrough(foldingDiskHolding(c.stored...), c.typed), true)
		})
	}
}
