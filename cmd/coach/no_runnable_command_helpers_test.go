package main

import (
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The advice for a step that overwrites or moves the user's files is told in words: Coach names the file and the step, and the
// user runs it. A command to paste that carries such a step out is a pattern here, not a fixed string, so a command written
// with options between git and its subcommand, with another run of whitespace between its words, or as a bare mv, cp or rm is
// caught as well as the commands the advice used to print. The clear-a-flag command (`git update-index`) is not among them: it
// only clears a flag, and the advice may print it.
// anOptionValue is the value of an option: a word, or a quoted run of words.
const anOptionValue = `(?:"[^"]*"|'[^']*'|\S+)`

var (
	// gitThatChangesFiles is git, any options before the subcommand, and a subcommand that moves, restores, stages, commits or
	// discards files. Case-insensitive, and any whitespace, newlines included, may separate the words. An option before the
	// subcommand is -c or -C with its value, one of git's long options that takes its value as the next word (--git-dir /x,
	// --work-tree /x), or any other long option with its value joined by = or none; a value may be quoted.
	gitThatChangesFiles = regexp.MustCompile(`(?i)\bgit\b(?:\s+(?:-[cC]\s+` + anOptionValue + `|--(?:git-dir|work-tree|namespace|super-prefix|config-env|attr-source|exec-path)\s+` + anOptionValue + `|--[\w-]+(?:=\S+)?))*\s+(?:mv|checkout|restore|rm|add|commit|reset|stash|switch|clean|apply|merge|rebase|cherry-pick)\b`)
	// aFileMovingProgram is mv, cp or rm run on an argument.
	aFileMovingProgram = regexp.MustCompile(`(?i)(?:^|[\s;&|(` + "`" + `'"])(?:mv|cp|rm)\s+\S`)
	// aShellExpansionOrAMergeFlag is what a pasted command carries to find the repository root or to pick a side of a conflict.
	aShellExpansionOrAMergeFlag = regexp.MustCompile(`--ours|--theirs|--merge\b|\$top|\$\(`)
)

var runnableCommandPatterns = []*regexp.Regexp{gitThatChangesFiles, aFileMovingProgram, aShellExpansionOrAMergeFlag}

var terminalEscapes = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// runnableCommandIn is the first command in text, with its terminal escapes removed, that carries out the step the advice
// describes, or "" when there is none.
func runnableCommandIn(text string) string {
	for _, pattern := range runnableCommandPatterns {
		if found := pattern.FindString(terminalEscapes.ReplaceAllString(text, "")); found != "" {
			return found
		}
	}
	return ""
}

// expectNoRunnableCommand fails when text holds any command that carries out the step it describes.
func expectNoRunnableCommand(text string) {
	GinkgoHelper()
	Expect(runnableCommandIn(text)).To(BeEmpty(), "the advice prints a command to paste; want the step described in words:\n%s", terminalEscapes.ReplaceAllString(text, ""))
}
