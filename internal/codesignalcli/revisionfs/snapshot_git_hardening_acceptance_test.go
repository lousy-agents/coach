package revisionfs

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("a snapshot read of a repository whose configuration names a program to run", func() {
	var dir, sha string

	BeforeEach(func() {
		dir = gitfixture.Init(GinkgoT())
		sha = gitfixture.CommitFile(GinkgoT(), dir, "main.go", "package main\n")
	})

	It("starts the git child with the arguments every other read of the repository starts with", func() {
		cmd := snapshotGitCommandContext(context.Background(), dir, "show", sha+":main.go")

		Expect(cmd.Args).To(Equal([]string{"git", "--no-optional-locks", "-C", dir, "-c", "core.fsmonitor=false", "show", sha + ":main.go"}))
	})

	DescribeTable("runs no core.fsmonitor program while listing and reading the tree",
		func(configure func(dir, program string)) {
			scripts := GinkgoT().TempDir()
			marker := filepath.Join(scripts, "fsmonitor-ran")
			program := filepath.Join(scripts, "fsmonitor.sh")
			Expect(os.WriteFile(program, []byte("#!/bin/sh\ntouch '"+marker+"'\nprintf '\\0'\n"), 0o755)).To(Succeed())
			configure(dir, program)

			fsys, err := New(dir, sha)
			Expect(err).NotTo(HaveOccurred())
			_, err = fs.ReadFile(fsys, "main.go")
			Expect(err).NotTo(HaveOccurred())

			Expect(marker).NotTo(BeAnExistingFile(), "git ran the core.fsmonitor program the configuration names")
		},
		Entry("named by the repository's own config", func(dir, program string) {
			Expect(exec.Command("git", "-C", dir, "config", "core.fsmonitor", program).Run()).To(Succeed())
		}),
		Entry("named by the environment's injected config", func(_, program string) {
			GinkgoT().Setenv("GIT_CONFIG_COUNT", "1")
			GinkgoT().Setenv("GIT_CONFIG_KEY_0", "core.fsmonitor")
			GinkgoT().Setenv("GIT_CONFIG_VALUE_0", program)
		}),
	)
})
