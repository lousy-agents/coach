package gitrepo

import "testing"

// git lists a name in the form the index holds it, composed or decomposed, and macOS precomposes what it is
// given, so a name typed one way can come back the other. The listing of a path that is a submodule returns the
// path itself; it is the path whichever Unicode form it is listed in.
func TestASubmoduleListedInAnotherUnicodeFormIsStillThePathItself(t *testing.T) {
	const composed = "vendér"
	const decomposed = "vendér"
	cases := map[string]struct{ typed, listed string }{
		"typed composed, listed decomposed": {composed, decomposed},
		"typed decomposed, listed composed": {decomposed, composed},
		"typed and listed the same":         {composed, composed},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheSubmoduleItself(t, c.typed, c.listed) })
	}
}

func requireTheSubmoduleItself(t *testing.T, typed, listed string) {
	t.Helper()
	run := func(string, ...string) ([]byte, error) { return []byte(listed + "\x00"), nil }

	below, itself, err := listBelow(run, "repo", typed)

	if err != nil || !below || !itself {
		t.Errorf("listBelow(%q) listed as %q = below %v, itself %v, %v; want the path itself", typed, listed, below, itself, err)
	}
}
