package invocation

import (
	"fmt"
	"path"

	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// samePath is what stands in for an authoring command whose --output path cannot
// be shown. The same command without the flag is a valid command that writes
// the policy somewhere else, so it is not printed.
const samePath = "run --suggest-project-config with --output naming the same path (its name is not printed here)"

// BatchAuthoring is the Go command that generates a candidate policy and writes
// it to target, or to authoring's default location when target is empty.
func BatchAuthoring(target string) string {
	if !shellword.Printable(target) {
		return samePath
	}
	command := "coach codesignal --baseline " + string(SuggestProjectConfig) + outputFlag(target)
	if note := directoryNote(target); note != "" {
		return note + " " + command
	}
	return command
}

// directoryNote is the condition under which authoring can write target. --output
// does not create a directory, so a target below the repository root needs its
// directory first. The note is true whether or not the directory exists, which the
// command alone could not be.
func directoryNote(target string) string {
	if dir := path.Dir(target); target != "" && dir != "." {
		return "after making sure " + shellword.Word(dir) + " exists at the repository root, run"
	}
	return ""
}

// outputFlag names where authoring writes the candidate: nothing for the default
// location, and the path as one shell word otherwise.
func outputFlag(target string) string {
	if target == "" {
		return ""
	}
	return " --output " + shellword.Word(target)
}

// ReplaceGoPolicy is the line for a Go policy that exists and cannot be used as one. Authoring creates files and
// refuses one that exists, so it writes a candidate beside the policy, at candidate, with Coach's own command, and the
// rest of the line is words: where the candidate is, which file to compare it with, and that the user replaces the policy
// with it, commits, and reruns. No command that moves or stages a file is printed, because the step overwrites the user's
// file and the user is the one to carry it out. Nothing is printed when either path cannot be shown: a line that names
// another path leads somewhere else.
func ReplaceGoPolicy(policyPath, candidate string) string {
	if !shellword.Printable(policyPath) || !shellword.Printable(candidate) {
		return ""
	}
	return BatchAuthoring(candidate) + fmt.Sprintf(
		" -- writes a candidate to %q beside the policy, because authoring refuses a path that exists; compare it with %q, replace the policy with it, commit, and rerun",
		candidate, policyPath)
}
