package projectconfig

import (
	"fmt"
	"strings"
	"testing"
)

const readPath = ".lousy-agents/coach.json"

// recordedAs is what the advice says to make git record, for the path Coach reads, whether the rename is yet to be
// made or is staged already.
func recordedAs(path string) string {
	return fmt.Sprintf("make git record it as %q (rename it in the repository, or commit a rename you have staged)", path)
}

func aRenameOverAFile(path string) string {
	return fmt.Sprintf("If another file already exists at %q, move it aside first; with core.ignorecase set, git can overwrite it", path)
}

// oneSpellingMessage is the whole message for a policy committed under one other spelling.
func oneSpellingMessage(committed, path string) string {
	return fmt.Sprintf("coach codesignal: the policy is committed as %q, but Coach reads %q, so it is not read: %s, then commit and run again. %s (project_config_invalid)", committed, path, recordedAs(path), aRenameOverAFile(path))
}

// severalMessage is the whole message for a policy committed under several other spellings: named is the quoted,
// sorted list, kept the spelling to keep, and deleted the quoted list of the others.
func severalMessage(named, kept, deleted string) string {
	return fmt.Sprintf("coach codesignal: the analyzed revision holds the policy under several spellings, %s, and Coach reads %q: keep one of them at that path and remove the others. To keep %q, %s, delete %s from the repository, then commit and run again. %s (project_config_invalid)", named, readPath, kept, recordedAs(readPath), deleted, aRenameOverAFile(readPath))
}

type refusalCase struct {
	name     string
	path     string
	revision revisionEntries
	want     string
}

func assertWrongCaseRefusal(t *testing.T, c refusalCase) {
	t.Helper()
	t.Run(c.name, func(t *testing.T) { assertTheRefusalIs(t, wrongCaseOf(c.revision, c.path), c.want) })
}

func assertTheRefusalIs(t *testing.T, verdict error, want string) {
	t.Helper()
	if verdict == nil {
		t.Fatalf("no refusal; want %q", want)
	}
	got := configErrorOf(t, verdict)
	if got.Kind != KindInvalid || string(got.PolicyCause) != "name_case_differs" || got.Remedy != 0 {
		t.Errorf("refusal = kind %v, cause %q, remedy %d; want an invalid policy with the name_case_differs cause and no remedy", got.Kind, got.PolicyCause, got.Remedy)
	}
	if got.Message != want {
		t.Errorf("message %q; want %q", got.Message, want)
	}
	if strings.ContainsAny(got.Message, "\x1b\n") || strings.Contains(got.Message, "git mv") || strings.Contains(got.Message, "$(") {
		t.Errorf("message %q holds a raw control character or a command to paste", got.Message)
	}
}

// The revision's listing is the only thing that says a policy is committed in another case, so the advice is the same
// for a policy Coach finds, one the user names and the project check, and for a disk of either kind: this asks nothing
// of a disk, an index or git's settings.
func TestAPolicyCommittedInAnotherCaseIsToldToMakeGitRecordItAsCoachReadsIt(t *testing.T) {
	for _, c := range []refusalCase{
		{"the directory and the file differ", readPath, fakeRevision{"": {".Lousy-Agents"}, ".Lousy-Agents": {"Coach.json"}}.entries,
			oneSpellingMessage(".Lousy-Agents/Coach.json", readPath)},
		{"only the directory differs", readPath, fakeRevision{"": {".Lousy-Agents"}, ".Lousy-Agents": {"coach.json"}}.entries,
			oneSpellingMessage(".Lousy-Agents/coach.json", readPath)},
		{"only the file differs", readPath, fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"Coach.json"}}.entries,
			oneSpellingMessage(".lousy-agents/Coach.json", readPath)},
		{"a policy at the root", "policy.json", fakeRevision{"": {"Policy.json"}}.entries,
			oneSpellingMessage("Policy.json", "policy.json")},
		{"a name with a control character, which is shown escaped", "policy\x1b.json", fakeRevision{"": {"Policy\x1b.json"}}.entries,
			oneSpellingMessage("Policy\x1b.json", "policy\x1b.json")},
	} {
		assertWrongCaseRefusal(t, c)
	}
}

func TestAPolicyCommittedUnderSeveralSpellingsNamesEveryOneAndHowToKeepOne(t *testing.T) {
	for _, c := range []refusalCase{
		{"one with the directory as Coach reads it, one with only the directory differing", readPath,
			fakeRevision{"": {".lousy-agents", ".LOUSY-AGENTS"}, ".lousy-agents": {"Coach.json"}, ".LOUSY-AGENTS": {"coach.json"}}.entries,
			severalMessage(`".LOUSY-AGENTS/coach.json", ".lousy-agents/Coach.json"`, ".lousy-agents/Coach.json", `".LOUSY-AGENTS/coach.json"`)},
		{"two spellings of the file in the one directory", readPath,
			fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"Coach.json", "COACH.JSON"}}.entries,
			severalMessage(`".lousy-agents/COACH.JSON", ".lousy-agents/Coach.json"`, ".lousy-agents/COACH.JSON", `".lousy-agents/Coach.json"`)},
		{"three spellings, which leave the repository together", readPath,
			fakeRevision{"": {".lousy-agents", ".LOUSY-AGENTS", ".Lousy-Agents"}, ".lousy-agents": {"Coach.json"}, ".LOUSY-AGENTS": {"coach.json"}, ".Lousy-Agents": {"COACH.JSON"}}.entries,
			severalMessage(`".LOUSY-AGENTS/coach.json", ".Lousy-Agents/COACH.JSON", ".lousy-agents/Coach.json"`, ".lousy-agents/Coach.json", `".LOUSY-AGENTS/coach.json", ".Lousy-Agents/COACH.JSON"`)},
	} {
		assertWrongCaseRefusal(t, c)
	}
}

func TestSeveralSpellingsOfAPolicyNamedByACharacterATerminalWouldActOnAreNamedEscaped(t *testing.T) {
	revision := fakeRevision{"": {"Policy\x1b.json", "POLICY\x1b.json"}}

	assertTheRefusalIs(t, wrongCaseOf(revision.entries, "policy\x1b.json"), fmt.Sprintf("coach codesignal: the analyzed revision holds the policy under several spellings, %q, %q, and Coach reads %q: keep one of them at that path and remove the others. To keep %q, %s, delete %q from the repository, then commit and run again. %s (project_config_invalid)",
		"POLICY\x1b.json", "Policy\x1b.json", "policy\x1b.json", "POLICY\x1b.json", recordedAs("policy\x1b.json"), "Policy\x1b.json", aRenameOverAFile("policy\x1b.json")))
}
