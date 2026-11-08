package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The guard against a partial clone's remote transport asks git whether the repository is a partial clone.
// A repository whose configuration git cannot read is not one Coach can call ordinary, so the scan stops
// there and says why, instead of reading on as if the answer were no.
var _ = Describe("coach codesignal: a repository whose configuration git cannot read", func() {
	It("is refused for want of knowing whether it is a partial clone", func() {
		repo := commitScannableGoRepo()
		config, err := os.OpenFile(filepath.Join(repo, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0o644)
		Expect(err).NotTo(HaveOccurred())
		_, err = config.WriteString("this line is not configuration\n")
		Expect(err).NotTo(HaveOccurred())
		Expect(config.Close()).To(Succeed())

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

		Expect(exitCode).To(Equal(1), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("could not tell whether this repository is a partial clone"), "stderr:\n%s", stderr)
	})
})
