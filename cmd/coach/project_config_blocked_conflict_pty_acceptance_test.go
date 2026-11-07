//go:build linux

package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: the conflicted policy behind a file, on a terminal", func() {
	It("--prepare-compiler names the move before any checkout", func() {
		repo := aConflictedPolicyBehindAFile("UD")

		stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(pathWithoutNode()), "", "codesignal", "--baseline", "--prepare-compiler", "--project-language", "typescript", "--project-config", conflictedPolicy)

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		expectEveryCheckoutBehindTheMove(string(stderr))
	})
})
