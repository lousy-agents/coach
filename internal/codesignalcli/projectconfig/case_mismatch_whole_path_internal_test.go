package projectconfig

import "testing"

// The pass clause names a path the revision holds in full: every component of the typed path is resolved against
// the revision, and not only the one the disk spelled differently. A path with two components typed in another
// case is advised as the revision holds both, and the advice pasted is accepted.
func TestTheAdviceNamesTheWholePathTheRevisionHolds(t *testing.T) {
	two := newCaseMismatch("CFG/P.json", []string{"CFG", "P.json"}, 0, "cfg")
	cases := []struct {
		name     string
		mismatch caseMismatchError
		entries  revisionEntries
		want     []string
		avoid    []string
	}{
		{"two components typed in another case", two, fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries,
			[]string{`it holds "cfg"; pass --project-config cfg/p.json`}, []string{"cfg/P.json"}},
		{"a later component the revision does not hold, so there is no path to pass", two, fakeRevision{"": {"cfg"}, "cfg": {"other.json"}}.entries,
			[]string{`it holds "cfg"`}, []string{"pass --project-config"}},
		{"a later component the revision holds in two spellings, so there is no one path to pass", two, fakeRevision{"": {"cfg"}, "cfg": {"p.json", "P.JSON"}}.entries,
			[]string{`it holds "cfg"`}, []string{"pass --project-config"}},
		{"a whole path held in two spellings", two, fakeRevision{"": {"cfg", "CFG"}, "cfg": {"p.json"}, "CFG": {"p.json"}}.entries,
			[]string{`it holds both "CFG" and "cfg"`}, []string{"pass --project-config"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertCaseMismatchAdvice(t, c.mismatch.config(c.entries), c.want, c.avoid)
		})
	}
}
