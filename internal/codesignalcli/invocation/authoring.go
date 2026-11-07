package invocation

import (
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
// refuses one that exists, so it writes a candidate beside the policy, at candidate, and the line gives the move that
// puts it over the policy and stages it. The move runs git at the repository root, so it works from any directory,
// quotes both paths for a shell, and stages the policy as a literal pathspec so that a glob character in its name
// matches that file and no other. It is the tail of the line, because a caller pastes everything after the lead-in.
// Nothing is printed when either path cannot be shown: a command that names another path writes somewhere else.
func ReplaceGoPolicy(policyPath, candidate string) string {
	if !shellword.Printable(policyPath) || !shellword.Printable(candidate) {
		return ""
	}
	return BatchAuthoring(candidate) +
		" -- writes a candidate beside the policy, because authoring refuses a path that exists; review it, then replace the policy with it, commit, and rerun. To replace it: " +
		`top="$(git rev-parse --show-toplevel)" && mv -f -- "$top"/` + shellword.Word(candidate) + ` "$top"/` + shellword.Word(policyPath) +
		` && git -C "$top" add -- ` + shellword.Word(":(literal)"+policyPath)
}
