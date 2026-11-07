package projectconfig

import "testing"

const (
	defaultSpelling = ".lousy-agents/coach.json"
	namedDefault    = ".lousy-agents/COACH.json"
)

// The default policy's path is the one a scan without --project-config reads, so a policy named with that path in another
// case is never told to be recorded at the spelling typed: the other callers would refuse it. Whole messages, by equality.
func TestTheDefaultPolicyNamedInAnotherCaseIsNotToldToRenameItToMatch(t *testing.T) {
	cases := []refusalCase{
		{"only the default spelling is committed", namedDefault, fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"coach.json"}}.entries, chosenByTheUser,
			`coach codesignal: the policy is committed as ".lousy-agents/coach.json", but Coach reads ".lousy-agents/COACH.json", so it is not read: pass the committed spelling, ".lousy-agents/coach.json", or leave --project-config off, which reads that spelling by itself; making git record it as ".lousy-agents/COACH.json" would make every scan without the flag refuse it (project_config_invalid)`},
		{"the default spelling is committed beside another", namedDefault, fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"coach.json", "Coach.json"}}.entries, chosenByTheUser,
			`coach codesignal: the analyzed revision holds the policy under several spellings, ".lousy-agents/Coach.json", ".lousy-agents/coach.json", and Coach reads ".lousy-agents/COACH.json": keep ".lousy-agents/coach.json", which a scan without --project-config reads by itself, and delete ".lousy-agents/Coach.json" from the repository, then commit and run again with that spelling passed or --project-config left off (project_config_invalid)`},
		{"only a spelling that is not the default is committed", namedDefault, fakeRevision{"": {".Lousy-Agents"}, ".Lousy-Agents": {"Coach.json"}}.entries, chosenByTheUser,
			oneSpellingMessageToDefault(".Lousy-Agents/Coach.json")},
	}
	for _, c := range cases {
		assertWrongCaseRefusal(t, c)
	}
}

// oneSpellingMessageToDefault is the message for a policy named with the default path in another case, committed under
// a spelling that is not the default: git is to record it at the default spelling, which both callers read.
func oneSpellingMessageToDefault(committed string) string {
	return "coach codesignal: the policy is committed as " + quoted(committed) + ", but Coach reads " + quoted(namedDefault) + ", so it is not read: pass the committed spelling, " + quoted(committed) + ", or " + recordedAs(defaultSpelling) + ", then commit and run again." + aDirectoryInCase + " " + aRenameOverAFile(defaultSpelling) + " (project_config_invalid)"
}

func quoted(s string) string { return `"` + s + `"` }

// Nothing changes for the policy Coach finds, or for a path that is not the default's: the spelling typed is the one
// Coach reads, and it is the one to record.
func TestOnlyTheDefaultPolicyNamedInAnotherCaseKeepsTheDefaultSpelling(t *testing.T) {
	for _, c := range []refusalCase{
		{"the policy Coach finds, with the default spelling not committed", defaultSpelling, fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"Coach.json"}}.entries, chosenByCoach,
			oneSpellingMessage(".lousy-agents/Coach.json", defaultSpelling, shape{})},
		{"the guard on who chose the path: a path Coach chose is never the user's to name", namedDefault, fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"coach.json"}}.entries, chosenByCoach,
			oneSpellingMessage(".lousy-agents/coach.json", namedDefault, shape{})},
		{"a path that is not the default's", "cfg/P.json", fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries, chosenByTheUser,
			oneSpellingMessage("cfg/p.json", "cfg/P.json", shape{named: true})},
	} {
		assertWrongCaseRefusal(t, c)
	}
}
