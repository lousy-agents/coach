package gitrepo

import (
	"errors"
	"reflect"
	"slices"
	"testing"
)

func records(parts ...string) []byte {
	var out []byte
	for _, part := range parts {
		out = append(out, part...)
		out = append(out, 0)
	}
	return out
}

func entryLine(path string) string {
	return "H 100644 " + listingBlob + " 0\t" + path
}

func answering(output []byte) Runner {
	return func(string, ...string) ([]byte, error) { return output, nil }
}

// answeringEach answers the two questions IndexEntries asks: the entries for the path itself
// (a --stage listing) and, when there are none, the paths below it.
func answeringEach(exact, below []byte) Runner {
	return func(_ string, args ...string) ([]byte, error) {
		if slices.Contains(args, "--stage") {
			return exact, nil
		}
		return below, nil
	}
}

type indexAtCase struct {
	exact, below []byte
	wantEntries  []IndexEntry
	wantBelow    bool
}

func requireIndexAt(t *testing.T, c indexAtCase) {
	t.Helper()
	got, err := IndexEntries(answeringEach(c.exact, c.below), "repo", "pol.json")
	if err != nil || !reflect.DeepEqual(got.Entries, c.wantEntries) || got.HoldsBelow != c.wantBelow {
		t.Errorf("IndexEntries = %+v, %v; want entries %v and HoldsBelow %v", got, err, c.wantEntries, c.wantBelow)
	}
}

func TestIndexEntriesSeparatesWhatIsAtThePathFromWhatIsBelowIt(t *testing.T) {
	atPath := []IndexEntry{{TreeEntry: TreeEntry{Mode: "100644", SHA: listingBlob}, Stage: "0"}}
	here := records(entryLine("pol.json"))
	cases := map[string]indexAtCase{
		"an entry for the path alone":                                   {here, nil, atPath, false},
		"files below the path, which makes it a directory in the index": {nil, records("pol.json/a.json"), nil, true},
		"nothing at the path and nothing below it":                      {nil, nil, nil, false},
		"an entry for the path wins, and files below it are not asked":  {here, records("pol.json/a.json"), atPath, false},
		"a name that git would quote without -z":                        {nil, records("pol.json/a\nb\"c.json"), nil, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireIndexAt(t, c) })
	}
}

func TestIndexEntriesMatchesAPathWhoseNameGitWouldQuote(t *testing.T) {
	const awkward = "dir/a\nb\tc\"d.json"

	got, err := IndexEntries(answering(records(entryLine(awkward))), "repo", awkward)

	if err != nil || len(got.Entries) != 1 || got.HoldsBelow {
		t.Errorf("IndexEntries = %+v, %v; want the one entry for %q: -z lists the name as it is, where a quoted name would never equal it", got, err, awkward)
	}
}

func requireOperationalError(t *testing.T, output []byte) {
	t.Helper()
	got, err := IndexEntries(answering(output), "repo", "pol.json")
	var operational *OperationalError
	if !errors.As(err, &operational) {
		t.Errorf("IndexEntries = %+v, %v; want an *OperationalError", got, err)
	}
}

func TestIndexEntriesRefusesAnEntryForAPathItWasNotAskedAbout(t *testing.T) {
	cases := map[string][]byte{
		"a sibling that shares the name as a prefix": records(entryLine("pol.jsonx")),
		"a parent":                                 records(entryLine("cfg")),
		"the same name in another case":            records(entryLine("POL.json")),
		"a good entry beside one for another path": records(entryLine("pol.json"), entryLine("other.json")),
	}
	for name, output := range cases {
		t.Run(name, func(t *testing.T) { requireOperationalError(t, output) })
	}
}

func TestIndexEntriesRefusesAnEmptyRecordInsideTheListing(t *testing.T) {
	requireOperationalError(t, records(entryLine("pol.json"), "", entryLine("pol.json")))
}
