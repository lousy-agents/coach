package gitrepo

import (
	"errors"
	"testing"
)

func lsTreeAnswer(record string) Runner {
	return func(string, ...string) ([]byte, error) { return []byte(record), nil }
}

// What the revision holds at a path decides whether it can be a policy at all, so the kind of the entry
// is read from the mode and type git records, and not inferred from the content.
func TestEntryKindAtRevisionReadsTheModeAndTypeGitRecords(t *testing.T) {
	const sha = "0123456789012345678901234567890123456789"
	cases := map[string]struct {
		record string
		want   EntryKind
	}{
		"nothing at the path":            {"", EntryAbsent},
		"a regular file":                 {"100644 blob " + sha + "\tcfg/p.json\x00", EntryRegularFile},
		"an executable file":             {"100755 blob " + sha + "\tcfg/p.json\x00", EntryRegularFile},
		"a symbolic link":                {"120000 blob " + sha + "\tcfg/p.json\x00", EntrySymbolicLink},
		"a directory":                    {"040000 tree " + sha + "\tcfg/p.json\x00", EntryDirectory},
		"a submodule":                    {"160000 commit " + sha + "\tcfg/p.json\x00", EntrySubmodule},
		"an entry of another kind":       {"100644 tag " + sha + "\tcfg/p.json\x00", EntryOther},
		"a commit that is not a gitlink": {"100644 commit " + sha + "\tcfg/p.json\x00", EntryOther},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheKind(t, c.record, c.want) })
	}
}

func requireTheKind(t *testing.T, record string, want EntryKind) {
	t.Helper()
	got, err := EntryKindAtRevision(lsTreeAnswer(record), "repo", "HEAD", "cfg/p.json")
	if err != nil || got != want {
		t.Errorf("EntryKindAtRevision = %v, %v; want %v", got, err, want)
	}
}

func TestEntryKindAtRevisionFailsClosed(t *testing.T) {
	cases := map[string]Runner{
		"git failing":                 func(string, ...string) ([]byte, error) { return nil, errors.New("boom") },
		"a record for another path":   lsTreeAnswer("100644 blob 0123456789012345678901234567890123456789\tother.json\x00"),
		"a record git does not print": lsTreeAnswer("not a record\x00"),
		"a record with a field more":  lsTreeAnswer("100644 blob 0123456789012345678901234567890123456789 extra\tcfg/p.json\x00"),
		"two records for the path":    lsTreeAnswer("100644 blob 0123456789012345678901234567890123456789\tcfg/p.json\x00040000 tree 0123456789012345678901234567890123456789\tcfg/p.json\x00"),
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) { requireAnOperationalError(t, run) })
	}
}

func requireAnOperationalError(t *testing.T, run Runner) {
	t.Helper()
	kind, err := EntryKindAtRevision(run, "repo", "HEAD", "cfg/p.json")
	var operational *OperationalError
	if !errors.As(err, &operational) || kind != EntryAbsent {
		t.Errorf("EntryKindAtRevision = %v, %v; want an *OperationalError and no kind", kind, err)
	}
}
