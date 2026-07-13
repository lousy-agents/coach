package main

import (
	"bytes"
	"errors"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal", func() {
	When("--base is not provided", func() {
		It("prints usage guidance to stderr and exits 2 without writing to stdout", func() {
			repo := newTempGitRepo()

			command := exec.Command(commandPath, "codesignal")
			command.Dir = repo
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()

			var exitErr *exec.ExitError
			Expect(err).To(HaveOccurred())
			Expect(errors.As(err, &exitErr)).To(BeTrue())
			Expect(exitErr.ExitCode()).To(Equal(2))
			Expect(stdout.Bytes()).To(BeEmpty())
			Expect(stderr.String()).NotTo(BeEmpty())
			Expect(stderr.String()).To(ContainSubstring("--base"))
		})
	})
})
