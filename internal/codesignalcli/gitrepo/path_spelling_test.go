package gitrepo

import "testing"

const (
	precomposed = "cfg/pé.json"  // é as one code point, the spelling git stores on macOS
	decomposed  = "cfg/pé.json" // e and a combining accent, the spelling a file system can hand back
)

// A path is one path whichever Unicode form it is spelled in: git on macOS precomposes the names it
// is given and lists, so the name asked about and the name listed can differ in bytes only. A name
// that differs in a letter or in case is another name.
func TestSamePathIgnoresOnlyTheUnicodeFormOfAName(t *testing.T) {
	cases := map[string]struct {
		a, b string
		want bool
	}{
		"the same bytes":                     {"cfg/p.json", "cfg/p.json", true},
		"precomposed and decomposed":         {precomposed, decomposed, true},
		"decomposed and precomposed":         {decomposed, precomposed, true},
		"another letter":                     {"cfg/p.json", "cfg/q.json", false},
		"another case":                       {"cfg/p.json", "CFG/p.json", false},
		"another accent":                     {precomposed, "cfg/pè.json", false},
		"a parent that is another directory": {"cfg/p.json", "cfg2/p.json", false},
		"nothing and a name":                 {"", "cfg", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireSamePath(t, c.a, c.b, c.want) })
	}
}

func requireSamePath(t *testing.T, a, b string, want bool) {
	t.Helper()
	if got := SamePath(a, b); got != want {
		t.Errorf("SamePath(%q, %q) = %v, want %v", a, b, got, want)
	}
}

func TestIsBelowIgnoresOnlyTheUnicodeFormOfTheDirectory(t *testing.T) {
	cases := map[string]struct {
		path, directory string
		want            bool
	}{
		"a file in the directory":                   {"cfg/p.json", "cfg", true},
		"the directory itself":                      {"cfg", "cfg", false},
		"a sibling whose name starts the same":      {"cfg2/p.json", "cfg", false},
		"a precomposed file in a decomposed parent": {"pé/a.json", "pé", true},
		"a decomposed file in a precomposed parent": {"pé/a.json", "pé", true},
		"another accent":                            {"pè/a.json", "pé", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireIsBelow(t, c.path, c.directory, c.want) })
	}
}

func requireIsBelow(t *testing.T, path, directory string, want bool) {
	t.Helper()
	if got := IsBelow(path, directory); got != want {
		t.Errorf("IsBelow(%q, %q) = %v, want %v", path, directory, got, want)
	}
}
