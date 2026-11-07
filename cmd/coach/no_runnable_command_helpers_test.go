package main

import (
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The advice for a step that overwrites or moves the user's files is told in words: Coach names the file and the
// step, and the user runs it. Every command a user could paste to carry such a step out is a token here.
var runnableCommandTokens = []string{
	"git mv", "git checkout", "git rm", "git restore", "mv -f", "rm --cached",
	"--ours", "--theirs", "--merge", "$top", "$(git", "git -C",
}

var terminalEscapes = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// expectNoRunnableCommand fails when text, with its terminal escapes removed, holds any command that carries out
// the step it describes.
func expectNoRunnableCommand(text string) {
	GinkgoHelper()
	plain := terminalEscapes.ReplaceAllString(text, "")
	for _, token := range runnableCommandTokens {
		Expect(strings.Contains(plain, token)).To(BeFalse(), "the advice prints %q, a command to paste; want the step described in words:\n%s", token, plain)
	}
}
