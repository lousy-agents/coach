package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// --project-config takes a path from the repository root, whatever directory the scan starts in. A path
// spelled from a subdirectory climbs out of the root and is refused; the message says how to spell it, and
// the scan does not suggest authoring, which would write a policy somewhere else and leave the path wrong.
var _ = Describe("coach codesignal: a --project-config path spelled from a subdirectory", func() {
	It("is refused with the way to spell it and no authoring suggestion", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, "policies/p.json", singleRootPolicyJSON)
		subdirectory := filepath.Join(repo, "sub", "deep")
		Expect(os.MkdirAll(subdirectory, 0o755)).To(Succeed())

		stdout, stderr, exitCode := runCoachBinary(commandPath, subdirectory, nil, "codesignal", "--baseline", "--project-config", "../../policies/p.json", "--format=json")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("from the repository root"), "stderr:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("--suggest-project-config"), "authoring does not fix a path:\n%s", stderr)
	})

	// Git tracks nothing under .git and refuses to add it, so advice to commit a file there, or to author a
	// policy there, sends the user in a circle.
	DescribeTable("refuses a path inside the .git directory, with no advice to commit, restore or author it",
		func(policy string, extraArgs ...string) {
			repo := commitScannableGoRepo()
			args := append([]string{"codesignal", "--baseline", "--project-config", policy}, extraArgs...)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, args...)

			refusal := withoutUsageLines(string(stderr))
			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(refusal).To(ContainSubstring(".git"), "stderr:\n%s", stderr)
			for _, advice := range []string{"commit the file", "--suggest-project-config", "git restore", "--output"} {
				Expect(refusal).NotTo(ContainSubstring(advice), "a path inside .git cannot be fixed by %q:\n%s", advice, stderr)
			}
		},
		Entry("a file git keeps there", ".git/config"),
		Entry("a file under a directory git keeps there", ".git/hooks/x"),
		Entry("the directory spelled in another case", ".GIT/config"),
		Entry("a file git keeps there, checked as a project", ".git/config", "--check-project", "--project-language", "typescript"),
	)
})

// withoutUsageLines is what a usage error says about the fault, without the synopsis that comes with it.
func withoutUsageLines(stderr string) string {
	var refusal []string
	for _, line := range strings.Split(stderr, "\n") {
		if !strings.HasPrefix(line, "usage:") && !strings.HasPrefix(line, "   or:") {
			refusal = append(refusal, line)
		}
	}
	return strings.Join(refusal, "\n")
}
