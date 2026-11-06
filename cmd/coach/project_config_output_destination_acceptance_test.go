//go:build unix

package main

import (
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --suggest-project-config --output: the destination named on the command line", func() {
	suggestRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", "module example.com/m\n\ngo 1.22\n")
		commitFile(repo, "a.go", minimalGoSource)
		return repo
	}

	It("refuses a destination whose parent directory does not exist, naming it", func() {
		repo := suggestRepo()

		_, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--suggest-project-config", "--output", "cfg/new.json")

		Expect(exitCode).To(Equal(2))
		Expect(string(stderr)).To(ContainSubstring(`parent directory \"cfg\" does not exist`))
	})

	It("refuses a FIFO destination without waiting on it", func() {
		repo := suggestRepo()
		Expect(syscall.Mkfifo(filepath.Join(repo, "f.json"), 0o644)).To(Succeed())

		start := time.Now()
		_, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--suggest-project-config", "--output", "f.json")

		Expect(exitCode).To(Equal(2))
		Expect(string(stderr)).To(ContainSubstring("--output target already exists"))
		Expect(time.Since(start)).To(BeNumerically("<", 10*time.Second))
	})

	It("creates a file under a name full of shell metacharacters and a newline, runs nothing, and escapes the name it reports", func() {
		repo := suggestRepo()
		name := "$(touch PWN)\nx.json"

		_, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--suggest-project-config", "--output", name)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(filepath.Join(repo, name)).To(BeARegularFile())
		Expect(filepath.Join(repo, "PWN")).NotTo(BeAnExistingFile())
		Expect(string(stderr)).NotTo(ContainSubstring("\n$(touch"), "the newline in the name reached the terminal raw")
	})
})
