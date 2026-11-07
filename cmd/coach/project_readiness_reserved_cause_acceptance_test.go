package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A --project-config value that cannot name a policy is refused with exit 2 and a message before any report
// exists, so there is no gap to carry a word for it. The cause vocabulary has no word for it either: the doc
// lists the values a consumer can match on, and none of them is for this refusal.
var _ = Describe("coach codesignal: a --project-config value that cannot name a policy", func() {
	DescribeTable("is refused with a message and no cause word",
		func(args ...string) {
			repo := commitScannableGoRepo()

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, args...)

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(string(stdout)+string(stderr)).NotTo(ContainSubstring("invalid_path"), "no cause word is declared for this refusal")
		},
		Entry("a path that climbs out of the repository, as text", "codesignal", "--baseline", "--project-config", "../x.json"),
		Entry("a path that climbs out of the repository, as JSON", "codesignal", "--baseline", "--project-config", "../x.json", "--format=json"),
		Entry("a path inside .git", "codesignal", "--baseline", "--project-config", ".git/config", "--format=json"),
		Entry("a path that climbs out of the repository, checked as a project", "codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "../x.json", "--format", "json"),
	)
})
