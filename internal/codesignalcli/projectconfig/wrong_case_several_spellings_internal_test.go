package projectconfig

import (
	"strings"
	"testing"
	"unicode"
)

// moveOneAndUnstageTheRest is the printed commands for several spellings: the one to keep is moved to the path
// Coach reads, and every other spelling leaves the index, so that nothing committed is left under another
// case. The others leave the index only: where a file system folds case they name the file just moved.
func moveOneAndUnstageTheRest(directory, keep, to string, others ...string) string {
	return moveFromTheRoot(directory, keep, to) + ` && git -C "$top" --literal-pathspecs rm --cached -q -- ` + strings.Join(others, " ")
}

func TestADiscoveredPolicyCommittedUnderSeveralSpellingsListsEveryOneAndMovesOneAndUnstagesTheRest(t *testing.T) {
	cases := []struct {
		name     string
		revision revisionEntries
		want     []string
		avoid    []string
	}{
		{"one with the directory as Coach reads it, one with only the directory differing",
			fakeRevision{"": {".lousy-agents", ".LOUSY-AGENTS"}, ".lousy-agents": {"Coach.json"}, ".LOUSY-AGENTS": {"coach.json"}}.entries,
			[]string{
				`several spellings, ".LOUSY-AGENTS/coach.json", ".lousy-agents/Coach.json", and Coach reads ".lousy-agents/coach.json"`,
				moveOneAndUnstageTheRest("'.lousy-agents'", "'.lousy-agents/Coach.json'", "'.lousy-agents/coach.json'", "'.LOUSY-AGENTS/coach.json'") + " and commit",
			}, nil},
		{"two spellings of the file in the one directory",
			fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"Coach.json", "COACH.JSON"}}.entries,
			[]string{
				`several spellings, ".lousy-agents/COACH.JSON", ".lousy-agents/Coach.json"`,
				moveOneAndUnstageTheRest("'.lousy-agents'", "'.lousy-agents/COACH.JSON'", "'.lousy-agents/coach.json'", "'.lousy-agents/Coach.json'") + " and commit",
			}, nil},
		{"three spellings, which leave the index together",
			fakeRevision{"": {".lousy-agents", ".LOUSY-AGENTS", ".Lousy-Agents"}, ".lousy-agents": {"Coach.json"}, ".LOUSY-AGENTS": {"coach.json"}, ".Lousy-Agents": {"COACH.JSON"}}.entries,
			[]string{
				`".LOUSY-AGENTS/coach.json", ".Lousy-Agents/COACH.JSON", ".lousy-agents/Coach.json"`,
				moveOneAndUnstageTheRest("'.lousy-agents'", "'.lousy-agents/Coach.json'", "'.lousy-agents/coach.json'", "'.LOUSY-AGENTS/coach.json' '.Lousy-Agents/COACH.JSON'"),
			}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertWrongCaseAdvice(t, c.revision, ".lousy-agents/coach.json", c.want, c.avoid)
		})
	}
}

func TestSeveralSpellingsOfAPolicyNamedByACharacterATerminalWouldActOnLeaveNoCommand(t *testing.T) {
	revision := fakeRevision{"": {"Policy\x1b.json", "POLICY\x1b.json"}}

	assertWrongCaseAdvice(t, revision.entries, "policy\x1b.json", []string{`several spellings, "POLICY\x1b.json", "Policy\x1b.json"`, "keep one of them at that path and remove the others"}, []string{" mv -- ", "rm --cached", "mkdir"})
}

// A listing could hold any number of spellings, and a message that names them all would not fit a terminal. Past
// the limit it says so and offers no command, since a command that left some out would leave them committed.
func TestADiscoveredPolicyCommittedUnderMoreSpellingsThanTheLimitIsToldSoAndGivenNoCommand(t *testing.T) {
	revision := fakeRevision{"": {".lousy-agents"}, ".lousy-agents": variantsOf("coach.json", maxSpellings+1)}

	assertWrongCaseAdvice(t, revision.entries, ".lousy-agents/coach.json", []string{"several spellings", "and more", "keep one of them at that path and remove the others"}, []string{" mv -- ", "rm --cached"})
}

// variantsOf is count names that differ from name only by case.
func variantsOf(name string, count int) []string {
	variants := make([]string, 0, count)
	for mask := 1; len(variants) < count; mask++ {
		if variant := upperCasedAt(name, mask); variant != name {
			variants = append(variants, variant)
		}
	}
	return variants
}

// upperCasedAt is name with the letters whose positions are set in mask in upper case.
func upperCasedAt(name string, mask int) string {
	runes := []rune(name)
	for i := range runes {
		if mask&(1<<i) != 0 {
			runes[i] = unicode.ToUpper(runes[i])
		}
	}
	return string(runes)
}

func TestTheSpellingKeptIsTheOneNeedingTheFewestRenamesAndTheDeepestFirstOne(t *testing.T) {
	cases := []struct {
		name      string
		typed     string
		spellings []string
		want      string
	}{
		{"fewer components differ", "a/b/c.json", []string{"A/B/c.json", "a/b/C.json"}, "a/b/C.json"},
		{"the first difference is deeper", "a/b/c.json", []string{"a/B/c.json", "A/b/c.json"}, "a/B/c.json"},
		{"the first difference is deeper though a later one is not", "a/b/c.json", []string{"A/b/C.json", "a/B/C.json"}, "a/B/C.json"},
		{"all else equal, the first in order", "x/y.json", []string{"x/y.JSON", "x/Y.json"}, "x/Y.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertSpellingKept(t, c.spellings, c.typed, c.want)
		})
	}
}

func assertSpellingKept(t *testing.T, spellings []string, typed, want string) {
	t.Helper()
	if got := keptSpelling(spellings, typed); got != want {
		t.Errorf("keptSpelling(%q, %q) = %q; want %q", spellings, typed, got, want)
	}
}

// The refusal names up to sixteen spellings, as the docs say, and the seventeenth makes it say there are more
// and print no command; the test names the number, so a limit changed in the code is seen.
func TestTheRefusalNamesExactlySixteenSpellingsBeforeItSaysThereAreMore(t *testing.T) {
	sixteen := fakeRevision{"": {".lousy-agents"}, ".lousy-agents": variantsOf("coach.json", 16)}
	seventeen := fakeRevision{"": {".lousy-agents"}, ".lousy-agents": variantsOf("coach.json", 17)}

	assertWrongCaseAdvice(t, sixteen.entries, ".lousy-agents/coach.json", []string{"several spellings", " mv -- "}, []string{"and more"})
	assertWrongCaseAdvice(t, seventeen.entries, ".lousy-agents/coach.json", []string{"several spellings", "and more"}, []string{" mv -- "})
}
