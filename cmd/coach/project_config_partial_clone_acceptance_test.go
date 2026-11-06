package main

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: a partial clone that lacks the policy's blob is read, never fetched into", func() {
	var clone, marker string
	var env []string

	DescribeTable("runs no program and writes no object, and declines cleanly",
		func(remoteURL func(bare string) string, scan bool) {
			sshCommand, sshMarker := sshProgramThatLeavesAMarker()
			marker = sshMarker
			env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCommand)
			clone = commitPartialClone(remoteURL)
			before := packFilesIn(clone)

			args := []string{"codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "cfg/p.json", "--format", "json"}
			want := "could not check whether it is committed at the analyzed revision"
			if scan {
				args = []string{"codesignal", "--baseline", "--project-config", "cfg/p.json", "--format=json", "--no-interactive"}
				want = "reading source snapshot"
			}
			stdout, stderr, exitCode := runCoachBinary(commandPath, clone, env, args...)

			Expect(exitCode).To(Equal(1), "an object that is not local is an operational failure:\nstdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring(want), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("is not committed"), "the policy is committed; its object is what is not here:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("git restore"), "no restore is offered for a blob that cannot be checked:\n%s", stderr)
			Expect(marker).NotTo(BeAnExistingFile(), "git ran the ssh program to fetch the object")
			Expect(packFilesIn(clone)).To(Equal(before), "the scan wrote objects into the repository")
		},
		Entry("--check-project, remote that would run a program", func(string) string { return "ssh://example.invalid/repo.git" }, false),
		Entry("--check-project, remote that would answer", func(bare string) string { return "file://" + bare }, false),
		Entry("a scan, remote that would run a program", func(string) string { return "ssh://example.invalid/repo.git" }, true),
		Entry("a scan, remote that would answer", func(bare string) string { return "file://" + bare }, true),
	)
})
