//go:build unix

package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree states how many unrecognized git warnings it could not read past (#168 item 22)", func() {
	unrecognizedLines := func(words ...string) []string {
		var lines []string
		for _, word := range words {
			lines = append(lines, "warning: "+word)
		}
		return lines
	}

	unrecognizedEntry := func(env []string) (scanIncompleteEntry, worktreeScanRun) {
		run := runWorktreeScan(scanRepoWith(nil), env, "--project-config", "project.json")
		return run.only("unrecognized_git_warning"), run
	}

	It("states a count of 1 when git printed one unrecognized warning, and a different count than for three (#168 item 22)", func() {
		one, oneRun := unrecognizedEntry(envWithStatusWarnings(unrecognizedLines("alpha unexpected")...))
		three, _ := unrecognizedEntry(envWithStatusWarnings(unrecognizedLines("alpha unexpected", "bravo unexpected", "charlie unexpected")...))

		Expect(one).To(HaveKeyWithValue("message", MatchRegexp(`\b1\b`)))
		Expect(one["message"]).NotTo(Equal(three["message"]))
		Expect(one).To(HaveKeyWithValue("hides_in_scope", "unknown"))
		Expect(one).NotTo(HaveKey("root"))
		Expect(oneRun.exitCode).To(Equal(3))
	})

	It("states a count of 3 in a single entry with no root and none of git's words when git printed three unrecognized warnings (#168 item 22)", func() {
		entry, run := unrecognizedEntry(envWithStatusWarnings(unrecognizedLines("alpha unexpected", "bravo unexpected", "charlie unexpected")...))

		Expect(entry).To(HaveKeyWithValue("message", MatchRegexp(`\b3\b`)))
		Expect(run.ofReason("unrecognized_git_warning")).To(HaveLen(1))
		Expect(entry).NotTo(HaveKey("root"))
		Expect(entry).To(HaveKeyWithValue("hides_in_scope", "unknown"))
		for _, word := range []string{"alpha", "bravo", "charlie", "unexpected", "warning:"} {
			Expect(entry["message"]).NotTo(ContainSubstring(word))
		}
		Expect(run.exitCode).To(Equal(3))
	})

	It("counts only the unrecognized warnings, not a recognized unreadable-directory warning printed beside them (#168 item 22)", func() {
		lines := append([]string{"warning: could not open directory 'pkg/y/': Permission denied"}, unrecognizedLines("alpha unexpected", "bravo unexpected")...)

		entry, run := unrecognizedEntry(envWithStatusWarnings(lines...))

		Expect(entry).To(HaveKeyWithValue("message", MatchRegexp(`\b2\b`)))
		Expect(entry["message"]).NotTo(MatchRegexp(`\b3\b`))
		Expect(run.only("unreadable_directory")).To(HaveKeyWithValue("root", "pkg/y"))
		Expect(run.exitCode).To(Equal(3))
	})
})
