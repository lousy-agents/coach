package projectconfig

import "fmt"
import "testing"

// severalToDefaultMessage is the whole message for the default policy's path named in another case, committed under
// several spellings none of which is the default: named is the quoted, sorted list, kept the spelling to keep, and
// deleted the quoted list of the others. One path is meant throughout the advice, the default spelling git is to
// record, and the next run is told what it takes to read the policy there.
func severalToDefaultMessage(named, kept, deleted string, s shape) string {
	return fmt.Sprintf("coach codesignal: the analyzed revision holds the policy under several spellings, %s, and Coach reads %q: pass one of the committed spellings, or keep one of them at %q, which a scan without --project-config reads by itself, and remove the others. To keep %q, %s, delete %s from the repository, %s.%s %s (project_config_invalid)",
		named, namedDefault, defaultSpelling, kept, recordedAs(defaultSpelling), deleted, runAgainWithTheDefault, s.directoryNote(), aRenameOverAFile(defaultSpelling))
}

// Recording the policy at the default spelling is what a policy named with the default's path in another case is told
// to do when none of the committed spellings is the default, so the whole advice names that spelling and not the one
// typed, and the run after it is told to pass that spelling or to leave the flag off: the flag as typed would refuse.
func TestSeveralSpellingsOfTheDefaultPolicyNamedInAnotherCaseAreToldToBeKeptAtTheDefaultSpelling(t *testing.T) {
	sameDirectory := fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"Coach.json", "COACH.JSON"}}.entries
	renamedDirectory := fakeRevision{"": {".LOUSY-AGENTS", ".Lousy-Agents"}, ".LOUSY-AGENTS": {"coach.json"}, ".Lousy-Agents": {"Coach.json"}}.entries
	for _, c := range []refusalCase{
		{"two spellings of the file in the default directory", namedDefault, sameDirectory, chosenByTheUser,
			severalToDefaultMessage(`".lousy-agents/COACH.JSON", ".lousy-agents/Coach.json"`, ".lousy-agents/COACH.JSON", `".lousy-agents/Coach.json"`, shape{})},
		{"the spelling to keep has a directory to rename", namedDefault, renamedDirectory, chosenByTheUser,
			severalToDefaultMessage(`".LOUSY-AGENTS/coach.json", ".Lousy-Agents/Coach.json"`, ".LOUSY-AGENTS/coach.json", `".Lousy-Agents/Coach.json"`, shape{directory: true})},
		{"the default directory holds one spelling and another directory holds the other", namedDefault, fakeRevision{"": {".Lousy-Agents", ".lousy-agents"}, ".Lousy-Agents": {"Coach.json"}, ".lousy-agents": {"Coach.json"}}.entries, chosenByTheUser,
			severalToDefaultMessage(`".Lousy-Agents/Coach.json", ".lousy-agents/Coach.json"`, ".lousy-agents/Coach.json", `".Lousy-Agents/Coach.json"`, shape{})},
		{"the spelling to keep is the one nearest the default spelling, not the one typed", namedDefault, fakeRevision{"": {".Lousy-Agents"}, ".Lousy-Agents": {"coach.json", "COACH.json"}}.entries, chosenByTheUser,
			severalToDefaultMessage(`".Lousy-Agents/COACH.json", ".Lousy-Agents/coach.json"`, ".Lousy-Agents/coach.json", `".Lousy-Agents/COACH.json"`, shape{directory: true})},
	} {
		assertWrongCaseRefusal(t, c)
	}
}

func TestMoreSpellingsOfTheDefaultPolicyThanTheLimitAreToldToBeKeptAtTheDefaultSpelling(t *testing.T) {
	revision := fakeRevision{"": {".lousy-agents"}, ".lousy-agents": variantsOf("coach.json", maxSpellings+1)}

	got := configErrorOf(t, wrongCaseOf(revision.entries, namedDefault, chosenByTheUser))

	requireTheMessageHas(t, got.Message, ", and more, and Coach reads "+quoted(namedDefault)+": pass one of the committed spellings, or keep one of them at "+quoted(defaultSpelling)+", which a scan without --project-config reads by itself, and remove the others, "+runAgainWithTheDefault+". "+aRenameOverAFile(defaultSpelling))
	requireTheMessageHasNot(t, got.Message, "that path", "To keep", "delete")
}
