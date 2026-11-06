package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// printedAuthoringCommand is the coach --suggest-project-config command a stderr
// prints, to the end of its line, or "" when none is printed.
func printedAuthoringCommand(stderr []byte) string {
	for _, line := range strings.Split(string(stderr), "\n") {
		if index := strings.Index(line, "coach codesignal --baseline --suggest-project-config"); index >= 0 {
			return strings.TrimSpace(line[index:])
		}
	}
	return ""
}

// createDirectoryTheLineNames does what the printed line asks before the command:
// --output does not create a directory, so a target below the repository root
// needs its directory first, and the line says so. A target authoring writes by
// default, or one at the root, names no directory.
func createDirectoryTheLineNames(repo, named string, stderr []byte) {
	directory := filepath.Dir(named)
	if directory == "." || named == ".lousy-agents/coach.json" {
		Expect(string(stderr)).NotTo(ContainSubstring("exists at the repository root"), "no directory is needed:\n%s", stderr)
		return
	}
	Expect(string(stderr)).To(ContainSubstring("exists at the repository root, run coach codesignal --baseline --suggest-project-config --output"),
		"--output does not create %q, so the line says it must exist first:\n%s", directory, stderr)
	Expect(os.MkdirAll(filepath.Join(repo, directory), 0o755)).To(Succeed())
}

// The authoring line beside a missing explicit policy is the other part of the
// message a user pastes. It has to write the policy where the user pointed
// --project-config, not where authoring writes by default; otherwise the command
// succeeds, the named path stays empty, and the rescan fails with the same error.
var _ = Describe("coach codesignal: the authoring command printed for a missing explicit policy", func() {
	DescribeTable("writes the policy at the path the user named",
		func(named string) {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "a.go", minimalGoSource)

			stderr := scanExplicitPolicy(repo, named)
			command := printedAuthoringCommand(stderr)
			Expect(command).NotTo(BeEmpty(), "a missing policy that nothing commits is offered authoring:\n%s", stderr)
			createDirectoryTheLineNames(repo, named, stderr)
			output, exitCode := runInShell(repo, "'"+commandPath+"'"+strings.TrimPrefix(command, "coach"))
			Expect(exitCode).To(Equal(0), "the printed command must work when pasted:\ncommand: %s\noutput: %s\nstderr:\n%s", command, output, stderr)

			Expect(filepath.Join(repo, named)).To(BeARegularFile(), "the command writes the policy at the named path:\ncommand: %s", command)
			Expect(filepath.Join(repo, "PWNED")).NotTo(BeAnExistingFile(), "a path the shell would expand is quoted:\ncommand: %s", command)
			gitIn(repo, "add", "-A")
			gitInWithIdentity(repo, "commit", "-q", "-m", "add the policy")
			_, rescanStderr, rescanExit := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", named, "--format=json")
			Expect(rescanExit).To(Equal(0), "once the written policy is committed the scan reads it:\n%s", rescanStderr)
		},
		Entry("a file at the repository root", "new.json"),
		Entry("a file in a directory", "cfg/new.json"),
		Entry("a path a shell would split", "cfg/a b.json"),
		Entry("a path a shell would expand", "cfg/$(touch PWNED).json"),
		Entry("the canonical path, which authoring writes by default", ".lousy-agents/coach.json"),
	)

	It("names the policy's path as --output beside the TypeScript guided-authoring command when no terminal is attached", func() {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "tsconfig.json", "{}\n")

		_, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-language", "typescript", "--no-interactive", "--project-config", "cfg/new.json", "--format=json")

		Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
		Expect(string(stderr)).To(ContainSubstring("once cfg exists at the repository root, run coach codesignal --baseline --suggest-project-config --project-language typescript --output cfg/new.json"), "stderr:\n%s", stderr)
	})

})
