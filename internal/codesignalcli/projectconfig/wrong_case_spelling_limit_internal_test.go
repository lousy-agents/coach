package projectconfig

import (
	"strings"
	"testing"
	"unicode"
)

// variantsOf is count names that differ from name only by case.
func variantsOf(name string, count int) []string {
	variants := make([]string, 0, count)
	for mask := 1; len(variants) < count; mask++ {
		variants = appendIfDifferent(variants, name, upperCasedAt(name, mask))
	}
	return variants
}

func appendIfDifferent(variants []string, name, variant string) []string {
	if variant == name {
		return variants
	}
	return append(variants, variant)
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

func requireTheMessageHas(t *testing.T, message string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(message, want) {
			t.Errorf("message %q; want it to contain %q", message, want)
		}
	}
}

func requireTheMessageHasNot(t *testing.T, message string, unwanted ...string) {
	t.Helper()
	for _, word := range unwanted {
		if strings.Contains(message, word) {
			t.Errorf("message %q must not contain %q", message, word)
		}
	}
}

// A listing could hold any number of spellings, and a message that names them all would not fit a terminal. Past the
// limit it says so and chooses none to keep, since a choice that left some out would leave them committed.
func TestAPolicyCommittedUnderMoreSpellingsThanTheLimitIsToldSoAndChosenNone(t *testing.T) {
	revision := fakeRevision{"": {".lousy-agents"}, ".lousy-agents": variantsOf("coach.json", maxSpellings+1)}

	got := configErrorOf(t, wrongCaseOf(revision.entries, readPath))

	requireTheMessageHas(t, got.Message, "several spellings", ", and more, and Coach reads", "keep one of them at that path and remove the others, then commit and run again. "+aRenameOverAFile(readPath))
	requireTheMessageHasNot(t, got.Message, "To keep", "delete")
}

// The refusal names up to sixteen spellings, as the docs say, and the seventeenth makes it say there are more and
// choose none to keep; the test names the number, so a limit changed in the code is seen.
func TestTheRefusalNamesExactlySixteenSpellingsBeforeItSaysThereAreMore(t *testing.T) {
	sixteen := fakeRevision{"": {".lousy-agents"}, ".lousy-agents": variantsOf("coach.json", 16)}
	seventeen := fakeRevision{"": {".lousy-agents"}, ".lousy-agents": variantsOf("coach.json", 17)}

	named := configErrorOf(t, wrongCaseOf(sixteen.entries, readPath)).Message
	more := configErrorOf(t, wrongCaseOf(seventeen.entries, readPath)).Message

	requireTheMessageHas(t, named, "To keep")
	requireTheMessageHasNot(t, named, "and more")
	requireTheMessageHas(t, more, "and more")
	requireTheMessageHasNot(t, more, "To keep")
}
