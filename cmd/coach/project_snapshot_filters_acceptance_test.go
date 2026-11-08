package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: a scan reads the committed bytes and runs no filter that a repository selects", func() {
	smudgeProgram := func() (script, marker string) {
		directory := GinkgoT().TempDir()
		marker = filepath.Join(directory, "smudged")
		script = filepath.Join(directory, "smudge.sh")
		Expect(os.WriteFile(script, []byte("#!/bin/sh\ntouch '"+marker+"'\ncat\n"), 0o755)).To(Succeed())
		return script, marker
	}

	It("runs no smudge driver that the repository's own configuration defines", func() {
		script, marker := smudgeProgram()
		repo := commitScannableGoRepo()
		commitFile(repo, ".gitattributes", "*.go filter=evil\n")
		gitIn(repo, "config", "filter.evil.smudge", script)

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, os.Environ(), "codesignal", "--baseline", "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(marker).NotTo(BeAnExistingFile(), "a smudge driver ran while the scan read the repository")
	})

	It("runs no smudge driver that the user's own configuration defines and a committed .gitattributes selects, as git-lfs is set up", func() {
		script, marker := smudgeProgram()
		globalConfig := filepath.Join(GinkgoT().TempDir(), "gitconfig")
		Expect(os.WriteFile(globalConfig, []byte("[filter \"lfs\"]\n\tsmudge = "+script+" %f\n\trequired = true\n"), 0o644)).To(Succeed())
		source := commitScannableGoRepo()
		commitFile(source, ".gitattributes", "* filter=lfs\n")
		clone := filepath.Join(GinkgoT().TempDir(), "clone")
		gitOutputIn(GinkgoT().TempDir(), "clone", "--quiet", source, clone)
		Expect(marker).NotTo(BeAnExistingFile(), "the clone ran the driver before the scan was started")

		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, append(os.Environ(), "GIT_CONFIG_GLOBAL="+globalConfig), "codesignal", "--baseline", "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(marker).NotTo(BeAnExistingFile(), "the user's lfs driver ran on a committed file while the scan read the repository")
	})
})
