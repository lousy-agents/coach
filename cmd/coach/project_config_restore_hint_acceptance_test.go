package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	restorePolicyName     = "pol.json"
	injectingPolicyName   = "$(touch PWNED).json"
	controlCharPolicyName = "bad\x1bname.json"
)

// restoreCommandFrom returns what the error offers after "restore it with ",
// the text a user would paste into a shell.
func restoreCommandFrom(stderr []byte) string {
	for _, line := range strings.Split(string(stderr), "\n") {
		if _, command, found := strings.Cut(line, "restore it with "); found {
			return command
		}
	}
	return ""
}

func runInShell(dir, command string) (output string, exitCode int) {
	shell := exec.Command("sh", "-c", command)
	shell.Dir = dir
	combined, err := shell.CombinedOutput()
	if err == nil {
		return string(combined), 0
	}
	exitErr, isExit := err.(*exec.ExitError)
	ExpectWithOffset(1, isExit).To(BeTrue(), "running %q: %v", command, err)
	return string(combined), exitErr.ExitCode()
}

func gitIn(repo string, args ...string) string {
	command := exec.Command("git", args...)
	command.Dir = repo
	output, err := command.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git %v: %s", args, output)
	return string(output)
}

var _ = Describe("coach codesignal: the restore command offered for a committed explicit policy that is gone from disk", func() {
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, restorePolicyName, singleRootPolicyJSON)
		commitFile(repo, injectingPolicyName, singleRootPolicyJSON)
		commitFile(repo, controlCharPolicyName, singleRootPolicyJSON)
	})

	scanFrom := func(dir, policy string) []byte {
		stdout, stderr, exitCode := runCoachBinary(commandPath, dir, nil, "codesignal", "--baseline", "--project-config", policy, "--format=json")
		ExpectWithOffset(1, exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		return stderr
	}

	It("restores the file when it is run from a subdirectory", func() {
		subdir := filepath.Join(repo, "sub")
		Expect(os.MkdirAll(subdir, 0o755)).To(Succeed())
		Expect(os.Remove(filepath.Join(repo, restorePolicyName))).To(Succeed())

		command := restoreCommandFrom(scanFrom(subdir, restorePolicyName))
		Expect(command).NotTo(BeEmpty(), "the error offers no command")
		output, exitCode := runInShell(subdir, command)

		Expect(exitCode).To(Equal(0), "command=%s output=%s", command, output)
		Expect(filepath.Join(repo, restorePolicyName)).To(BeARegularFile())
	})

	It("restores the file after the deletion was staged with git rm", func() {
		gitIn(repo, "rm", "-q", restorePolicyName)

		command := restoreCommandFrom(scanFrom(repo, restorePolicyName))
		Expect(command).NotTo(BeEmpty(), "the error offers no command")
		output, exitCode := runInShell(repo, command)

		Expect(exitCode).To(Equal(0), "command=%s output=%s", command, output)
		Expect(filepath.Join(repo, restorePolicyName)).To(BeARegularFile())
		Expect(gitIn(repo, "status", "--porcelain", "--", restorePolicyName)).To(BeEmpty(), "both the index and the file are back")
	})

	It("cannot be made to run another command by a file name", func() {
		Expect(os.Remove(filepath.Join(repo, injectingPolicyName))).To(Succeed())

		command := restoreCommandFrom(scanFrom(repo, injectingPolicyName))
		Expect(command).NotTo(BeEmpty(), "the error offers no command")
		output, exitCode := runInShell(repo, command)

		Expect(exitCode).To(Equal(0), "command=%s output=%s", command, output)
		Expect(filepath.Join(repo, "PWNED")).NotTo(BeAnExistingFile(), "the shell ran the file name as a command")
		Expect(filepath.Join(repo, injectingPolicyName)).To(BeARegularFile())
	})

	It("offers no command for a name with a control character, and prints none of it raw", func() {
		Expect(os.Remove(filepath.Join(repo, controlCharPolicyName))).To(Succeed())

		stderr := scanFrom(repo, controlCharPolicyName)

		Expect(string(stderr)).NotTo(ContainSubstring("git restore"))
		Expect(string(stderr)).To(ContainSubstring("restore it from git at the analyzed revision"))
		Expect(string(stderr)).NotTo(ContainSubstring("\x1b"), "the escape character reached the terminal")
	})
})
