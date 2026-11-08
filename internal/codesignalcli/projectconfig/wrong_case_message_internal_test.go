package projectconfig

import (
	"fmt"
	"strings"
	"testing"
)

const readPath = ".lousy-agents/coach.json"

// aDirectoryInCase is the sentence a rename of a directory in case only carries, and is the only text it adds.
const aDirectoryInCase = " A directory whose name changes only in case cannot be renamed in one step on a disk that folds case: rename it to another name first, then to the one you want."

// shape is what varies in a message besides the spellings: whether the policy was named by the user, who may pass the
// committed spelling instead, and whether the rename changes a directory's case.
type shape struct {
	named, directory bool
}

func (s shape) directoryNote() string {
	if s.directory {
		return aDirectoryInCase
	}
	return ""
}

// recordedAs is what the advice says to make git record, for the path Coach reads, whether the rename is yet to be
// made or is staged already.
func recordedAs(path string) string {
	return fmt.Sprintf("make git record it as %q (rename it in the repository, or commit a rename you have staged)", path)
}

func aRenameOverAFile(path string) string {
	return fmt.Sprintf("If another file already exists at %q, move it aside first; with core.ignorecase set, git can overwrite it", path)
}

// oneSpellingMessage is the whole message for a policy committed under one other spelling.
func oneSpellingMessage(committed, path string, s shape) string {
	lead := ""
	if s.named {
		lead = fmt.Sprintf("pass the committed spelling, %q, or ", committed)
	}
	return fmt.Sprintf("coach codesignal: the policy is committed as %q, but Coach reads %q, so it is not read: %s%s, then commit and run again.%s %s (project_config_invalid)", committed, path, lead, recordedAs(path), s.directoryNote(), aRenameOverAFile(path))
}

// severalMessage is the whole message for a policy committed under several other spellings: named is the quoted,
// sorted list, kept the spelling to keep, and deleted the quoted list of the others.
func severalMessage(named, kept, deleted string, s shape) string {
	lead := ""
	if s.named {
		lead = "pass one of the committed spellings, or "
	}
	return fmt.Sprintf("coach codesignal: the analyzed revision holds the policy under several spellings, %s, and Coach reads %q: %skeep one of them at that path and remove the others. To keep %q, %s, delete %s from the repository, then commit and run again.%s %s (project_config_invalid)", named, readPath, lead, kept, recordedAs(readPath), deleted, s.directoryNote(), aRenameOverAFile(readPath))
}

type refusalCase struct {
	name     string
	path     string
	revision revisionEntries
	chooser  pathChooser
	want     string
}

func assertWrongCaseRefusal(t *testing.T, c refusalCase) {
	t.Helper()
	t.Run(c.name, func(t *testing.T) { assertTheRefusalIs(t, wrongCaseOf(c.revision, c.path, c.chooser), c.want) })
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
// of a disk, an index or git's settings. A policy the user names has one more way out, which is to pass the spelling
// that is committed.
func TestAPolicyCommittedInAnotherCaseIsToldToMakeGitRecordItAsCoachReadsIt(t *testing.T) {
	both := fakeRevision{"": {".Lousy-Agents"}, ".Lousy-Agents": {"Coach.json"}}.entries
	file := fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"Coach.json"}}.entries
	for _, c := range []refusalCase{
		{"the directory and the file differ", readPath, both, chosenByCoach, oneSpellingMessage(".Lousy-Agents/Coach.json", readPath, shape{directory: true})},
		{"only the directory differs", readPath, fakeRevision{"": {".Lousy-Agents"}, ".Lousy-Agents": {"coach.json"}}.entries, chosenByCoach, oneSpellingMessage(".Lousy-Agents/coach.json", readPath, shape{directory: true})},
		{"only the file differs", readPath, file, chosenByCoach, oneSpellingMessage(".lousy-agents/Coach.json", readPath, shape{})},
		{"a policy at the root", "policy.json", fakeRevision{"": {"Policy.json"}}.entries, chosenByCoach, oneSpellingMessage("Policy.json", "policy.json", shape{})},
		{"a name with a control character, which is shown escaped", "policy\x1b.json", fakeRevision{"": {"Policy\x1b.json"}}.entries, chosenByCoach, oneSpellingMessage("Policy\x1b.json", "policy\x1b.json", shape{})},
		{"named: the directory and the file differ", readPath, both, chosenByTheUser, oneSpellingMessage(".Lousy-Agents/Coach.json", readPath, shape{named: true, directory: true})},
		{"named: only the file differs", readPath, file, chosenByTheUser, oneSpellingMessage(".lousy-agents/Coach.json", readPath, shape{named: true})},
	} {
		assertWrongCaseRefusal(t, c)
	}
}

func TestAPolicyCommittedUnderSeveralSpellingsNamesEveryOneAndHowToKeepOne(t *testing.T) {
	keptHere := fakeRevision{"": {".lousy-agents", ".LOUSY-AGENTS"}, ".lousy-agents": {"Coach.json"}, ".LOUSY-AGENTS": {"coach.json"}}.entries
	keptElsewhere := fakeRevision{"": {".LOUSY-AGENTS", ".Lousy-Agents"}, ".LOUSY-AGENTS": {"coach.json"}, ".Lousy-Agents": {"Coach.json"}}.entries
	for _, c := range []refusalCase{
		{"one with the directory as Coach reads it, one with only the directory differing", readPath, keptHere, chosenByCoach,
			severalMessage(`".LOUSY-AGENTS/coach.json", ".lousy-agents/Coach.json"`, ".lousy-agents/Coach.json", `".LOUSY-AGENTS/coach.json"`, shape{})},
		{"two spellings of the file in the one directory", readPath, fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"Coach.json", "COACH.JSON"}}.entries, chosenByCoach,
			severalMessage(`".lousy-agents/COACH.JSON", ".lousy-agents/Coach.json"`, ".lousy-agents/COACH.JSON", `".lousy-agents/Coach.json"`, shape{})},
		{"three spellings, which leave the repository together", readPath,
			fakeRevision{"": {".lousy-agents", ".LOUSY-AGENTS", ".Lousy-Agents"}, ".lousy-agents": {"Coach.json"}, ".LOUSY-AGENTS": {"coach.json"}, ".Lousy-Agents": {"COACH.JSON"}}.entries, chosenByCoach,
			severalMessage(`".LOUSY-AGENTS/coach.json", ".Lousy-Agents/COACH.JSON", ".lousy-agents/Coach.json"`, ".lousy-agents/Coach.json", `".LOUSY-AGENTS/coach.json", ".Lousy-Agents/COACH.JSON"`, shape{})},
		{"the spelling to keep has a directory to rename", readPath, keptElsewhere, chosenByCoach,
			severalMessage(`".LOUSY-AGENTS/coach.json", ".Lousy-Agents/Coach.json"`, ".LOUSY-AGENTS/coach.json", `".Lousy-Agents/Coach.json"`, shape{directory: true})},
		{"named: one with the directory as Coach reads it", readPath, keptHere, chosenByTheUser,
			severalMessage(`".LOUSY-AGENTS/coach.json", ".lousy-agents/Coach.json"`, ".lousy-agents/Coach.json", `".LOUSY-AGENTS/coach.json"`, shape{named: true})},
		{"named: the spelling to keep has a directory to rename", readPath, keptElsewhere, chosenByTheUser,
			severalMessage(`".LOUSY-AGENTS/coach.json", ".Lousy-Agents/Coach.json"`, ".LOUSY-AGENTS/coach.json", `".Lousy-Agents/Coach.json"`, shape{named: true, directory: true})},
	} {
		assertWrongCaseRefusal(t, c)
	}
}

func TestSeveralSpellingsOfAPolicyNamedByACharacterATerminalWouldActOnAreNamedEscaped(t *testing.T) {
	revision := fakeRevision{"": {"Policy\x1b.json", "POLICY\x1b.json"}}

	assertTheRefusalIs(t, wrongCaseOf(revision.entries, "policy\x1b.json", chosenByCoach), fmt.Sprintf("coach codesignal: the analyzed revision holds the policy under several spellings, %q, %q, and Coach reads %q: keep one of them at that path and remove the others. To keep %q, %s, delete %q from the repository, then commit and run again. %s (project_config_invalid)",
		"POLICY\x1b.json", "Policy\x1b.json", "policy\x1b.json", "POLICY\x1b.json", recordedAs("policy\x1b.json"), "Policy\x1b.json", aRenameOverAFile("policy\x1b.json")))
}
