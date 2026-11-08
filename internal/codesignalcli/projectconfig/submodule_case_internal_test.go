package projectconfig

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// theIndexRecords is a fake git for the submodule lookup: it lists the submodules recorded, answers core.ignorecase with
// what it was given, and notes what it was asked. Any other question is an error that names it.
type theIndexRecords struct {
	submodules []string
	ignoreCase string
	configErr  error
	asked      []string
	pathspecs  []string
}

func (g *theIndexRecords) run(_ string, args ...string) ([]byte, error) {
	g.asked = append(g.asked, args[0])
	switch args[0] {
	case "ls-files":
		g.pathspecs = append(g.pathspecs, args[len(args)-1])
		var listing strings.Builder
		for _, name := range g.submodules {
			listing.WriteString("160000 " + committedBlob + " 0\t" + name + "\x00")
		}
		return []byte(listing.String()), nil
	case "config":
		if g.configErr != nil {
			return nil, g.configErr
		}
		if g.ignoreCase == "" {
			return nil, gitSaysUnset
		}
		return []byte(g.ignoreCase + "\n"), nil
	}
	return nil, errors.New("unexpected git question: " + strings.Join(args, " "))
}

func (g *theIndexRecords) askedAbout(question string) bool { return slices.Contains(g.asked, question) }

// A submodule recorded as `vendor/sub` is the one a name in another case points at, where git ignores case: the lookup
// matches without regard to case, and gives the spelling the index holds.
func TestASubmoduleRecordedInAnotherCaseIsTheOneAskedForWhereGitIgnoresCase(t *testing.T) {
	git := &theIndexRecords{submodules: []string{"vendor/sub"}, ignoreCase: "true"}

	recorded, err := submoduleRecordedAs(git.run, "repo", "Vendor/Sub")

	if err != nil || recorded != "vendor/sub" {
		t.Errorf("submoduleRecordedAs = %q, %v; want the spelling the index holds", recorded, err)
	}
	if !slices.Contains(git.pathspecs, ":(top,literal,icase)Vendor/Sub") {
		t.Errorf("asked %q; want a pathspec that matches without regard to case", git.pathspecs)
	}
}

// Where git tells cases apart, `Sub` is not `sub`: a name in another case is a different path, and an unset
// core.ignorecase is git's plain no.
func TestASubmoduleRecordedInAnotherCaseIsNotTheOneAskedForWhereGitTellsCasesApart(t *testing.T) {
	for name, git := range map[string]*theIndexRecords{
		"core.ignorecase is false": {submodules: []string{"vendor/sub"}, ignoreCase: "false"},
		"core.ignorecase is unset": {submodules: []string{"vendor/sub"}},
	} {
		t.Run(name, func(t *testing.T) { requireNoSubmoduleRecordedAs(t, git, "Vendor/Sub") })
	}
}

func requireNoSubmoduleRecordedAs(t *testing.T, git *theIndexRecords, path string) {
	t.Helper()
	recorded, err := submoduleRecordedAs(git.run, "repo", path)

	if err != nil || recorded != "" {
		t.Errorf("submoduleRecordedAs = %q, %v; want none", recorded, err)
	}
}
