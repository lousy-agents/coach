package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --help describing where --project-config is read from (#168)", func() {
	It("says a scan reads the policy from disk and discloses it against HEAD, not that it is read at the selected revision", func() {
		stdout, stderr, exitCode := runCoachRaw("codesignal", "--help")

		help := string(stdout) + string(stderr)
		Expect(exitCode).To(Equal(0))
		Expect(help).To(MatchRegexp(`(?s)-project-config string\n\s+[^\n]*read from disk[^\n]*HEAD`))
		Expect(help).NotTo(ContainSubstring("read at the selected revision"))
	})
})
