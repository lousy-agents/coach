package main

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: a committed policy that is evil is refused with a reason, and promptly", func() {
	DescribeTable("the committed bytes are not a usable policy",
		func(content, reason string) {
			repo := commitScannableGoRepo()
			commitFile(repo, explicitPolicyPath, content)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
			Expect(string(stderr)).To(ContainSubstring(reason))
		},
		Entry("random binary", "\x00\xff\xfe\x80binary\x00\x01", "document is not valid JSON"),
		Entry("zero bytes", "", "document is not valid JSON"),
		Entry("a byte-order mark before the JSON", "\xef\xbb\xbf"+singleRootPolicyJSON, "document is not valid JSON"),
		Entry("200000 levels of nesting", strings.Repeat("[", 200000)+strings.Repeat("]", 200000), "document is not valid JSON"),
		Entry("a duplicated object key", `{"schema_version":"1","roots":["."],"roots":["../../etc"]}`, `duplicate object key "roots"`),
		Entry("a root that leaves the repository", `{"schema_version":"1","roots":["../../etc"]}`, `root "../../etc": must be a normalized repository-relative path`),
	)

	It("refuses a committed policy over the output budget within seconds, not at the git time bound", func() {
		// well past the budget: git can only be left blocked writing when more is unread than a pipe holds
		repo := commitScannableGoRepo()
		commitFile(repo, explicitPolicyPath, strings.Repeat(" ", 2<<20)+singleRootPolicyJSON)

		start := time.Now()
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("git stdout exceeded 1048576-byte budget"))
		Expect(time.Since(start)).To(BeNumerically("<", 10*time.Second), "the refusal waited for git's 30-second bound")
	})
})
