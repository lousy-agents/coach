package main

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: a policy whose content a partial clone lacks is explained", func() {
	It("says the policy's content is not in the partial clone and names the command that fetches it", func() {
		skipUnlessGitCanRefuseLazyFetch()
		clone := commitPartialClone(func(string) string { return "ssh://example.invalid/repo.git" })
		revision := strings.TrimSpace(gitOutputIn(clone, "rev-parse", "HEAD"))

		_, stderr, exitCode := runCoachBinary(commandPath, clone, os.Environ(), "codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "cfg/p.json", "--format", "json")

		Expect(exitCode).To(Equal(1), "stderr: %s", stderr)
		Expect(string(stderr)).To(ContainSubstring("its content is not in this partial clone, and Coach does not fetch objects"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("git cat-file -e '"+revision+":cfg/p.json'"), "stderr:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("git said"), "the words are Coach's, not git's:\n%s", stderr)
	})
})
