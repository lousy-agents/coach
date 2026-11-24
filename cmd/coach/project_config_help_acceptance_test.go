package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --help describes where the --project-config policy is read from (#168)", func() {
	It("says the content is read from disk and that scope.policy discloses a difference from HEAD, not that it is read at the revision", func() {
		stdout, stderr, exitCode := runCoachRaw("codesignal", "--help")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		help := string(stdout) + string(stderr)
		Expect(help).To(ContainSubstring("its content is read from disk; scope.policy says whether it matches HEAD's copy"))
		Expect(help).NotTo(ContainSubstring("read at the selected revision"))
	})
})
