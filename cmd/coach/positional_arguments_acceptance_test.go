package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func repoWithOneCommit() string {
	repo := newTempGitRepo()
	commitFile(repo, "a.go", "package a\n")
	return repo
}

func repoWithTwoCommits() string {
	repo := newTempGitRepo()
	commitFile(repo, "a.go", "package a\n")
	commitFile(repo, "b.go", "package b\n")
	return repo
}

var _ = Describe("coach codesignal on the main scan path", func() {
	When("the invocation carries one or more positional arguments", func() {
		DescribeTable("exits 2, writes nothing to stdout, and writes a usage error that quotes the unexpected argument",
			func(prepare func() string, invoke func(repo string) (stdout, stderr []byte, exitCode int), quoted, absent string) {
				repo := prepare()
				stdout, stderr, exitCode := invoke(repo)

				Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
				Expect(stdout).To(BeEmpty())
				Expect(string(stderr)).To(ContainSubstring("usage: coach codesignal"))
				Expect(string(stderr)).To(ContainSubstring(quoted))
				Expect(string(stderr)).NotTo(ContainSubstring("project_config_suggestion_invalid_arguments"))
				if absent != "" {
					Expect(string(stderr)).NotTo(ContainSubstring(absent))
				}
			},
			Entry("after --baseline",
				repoWithOneCommit,
				func(repo string) ([]byte, []byte, int) {
					return runCoachCodesignalBaselineRaw(repo, "extra-positional-arg")
				},
				`"extra-positional-arg"`,
				"",
			),
			Entry("after --baseline when --format json follows the positional, rejecting that flag rather than emitting a report",
				repoWithOneCommit,
				func(repo string) ([]byte, []byte, int) {
					return runCoachCodesignalBaselineRaw(repo, "extra-positional-arg", "--format", "json")
				},
				`"extra-positional-arg"`,
				"",
			),
			Entry("after --base on a repository where HEAD~1 is a real comparison",
				repoWithTwoCommits,
				func(repo string) ([]byte, []byte, int) {
					return runCoachCodesignalRaw(repo, "HEAD~1", "bogus")
				},
				`"bogus"`,
				"",
			),
			Entry("quoting the first residual token when two positionals follow --baseline",
				repoWithOneCommit,
				func(repo string) ([]byte, []byte, int) {
					return runCoachCodesignalBaselineRaw(repo, "foo", "bar")
				},
				`"foo"`,
				`"bar"`,
			),
		)
	})
})
