package main

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The user is often not at the repository root when they paste what Coach prints. The path Coach names is
// repository-relative, as --project-config is, so a command that resolves it against the working directory
// fails ("bad source") from a subdirectory, and the mkdir before it leaves a stray directory there.
func pasteIn(directory, command string, shell string) string {
	run := exec.Command(shell, "-c", command)
	run.Dir = directory
	output, err := run.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "the printed command must work from %s under %s:\n%s\n%s", directory, shell, command, output)
	return string(output)
}

// skipUnlessInstalled skips a spec for a shell that is not on this machine; sh always is.
func skipUnlessInstalled(shell string) {
	if _, err := exec.LookPath(shell); err != nil {
		Skip(shell + " is not installed here")
	}
}

var _ = Describe("coach codesignal: the rename a refusal prints works from a subdirectory", func() {
	DescribeTable("a policy committed in the wrong case, pasted from src/",
		func(shell string) {
			skipUnlessInstalled(shell)
			repo := commitScannableGoRepo()
			commitFile(repo, "src/keep.go", minimalGoSource)
			commitFile(repo, ".Lousy-Agents/Coach.json", singleRootPolicyJSON)
			subdirectory := filepath.Join(repo, "src")
			scan := func() (stdout, stderr []byte, exitCode int) {
				return runCoachBinary(commandPath, subdirectory, nil, "codesignal", "--baseline", "--format=json", "--no-interactive")
			}

			stdout, stderr, exitCode := scan()
			Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
			command := regexp.MustCompile(`rename it with (.+?) and commit the rename`).FindStringSubmatch(string(stderr))
			Expect(command).NotTo(BeNil(), "the message prints the commands:\n%s", stderr)

			pasteIn(subdirectory, command[1], shell)

			Expect(gitOutputIn(repo, "ls-files")).To(ContainSubstring(".lousy-agents/coach.json"))
			Expect(gitOutputIn(repo, "ls-files")).NotTo(ContainSubstring("Coach.json"))
			Expect(filepath.Join(subdirectory, ".lousy-agents")).NotTo(BeADirectory(), "the command left a stray directory in the subdirectory")
			gitInWithIdentity(repo, "commit", "-q", "-m", "move the policy to the path Coach reads")
			stdout, stderr, exitCode = scan()
			Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		},
		Entry("under sh", "sh"),
		Entry("under zsh", "zsh"),
	)

	DescribeTable("a policy renamed in case only on a file system that ignores case, pasted from src/",
		func(shell string) {
			skipUnlessInstalled(shell)
			skipUnlessFilesystemResolvesCase()
			repo := commitScannableGoRepo()
			commitFile(repo, "src/keep.go", minimalGoSource)
			commitFile(repo, "policies/p.json", singleRootPolicyJSON)
			renameInCaseOnly(filepath.Join(repo, "policies", "p.json"), filepath.Join(repo, "policies", "P.json"))
			subdirectory := filepath.Join(repo, "src")
			scan := func() (stdout, stderr []byte, exitCode int) {
				return runCoachBinary(commandPath, subdirectory, nil, "codesignal", "--baseline", "--project-config", "policies/P.json", "--format=json", "--no-interactive")
			}

			stdout, stderr, exitCode := scan()
			Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
			command := regexp.MustCompile(`record the rename with (.+?) and commit it`).FindStringSubmatch(string(stderr))
			Expect(command).NotTo(BeNil(), "the message prints a git mv command:\n%s", stderr)

			pasteIn(subdirectory, command[1], shell)
			gitInWithIdentity(repo, "commit", "-q", "-m", "rename the policy")

			stdout, stderr, exitCode = scan()
			Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(strings.TrimSpace(gitOutputIn(repo, "ls-files", "policies"))).To(Equal("policies/P.json"))
		},
		Entry("under sh", "sh"),
		Entry("under zsh", "zsh"),
	)
})

// The hint that moves a deprecated project.json names repository-relative paths too.
var _ = Describe("coach codesignal: the move of a deprecated project.json that a scan prints works from a subdirectory", func() {
	DescribeTable("pasted from src/",
		func(shell string) {
			skipUnlessInstalled(shell)
			repo := commitScannableGoRepo()
			commitFile(repo, "src/keep.go", minimalGoSource)
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
			subdirectory := filepath.Join(repo, "src")

			stdout, stderr, exitCode := runCoachBinary(commandPath, subdirectory, nil, "codesignal", "--baseline", "--format=json", "--no-interactive")
			Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
			skipped, ok := diagnosticFor(decodeCoachReport(stdout), "project_config_migration_skipped", legacyConfigPath)
			Expect(ok).To(BeTrue(), "stdout:\n%s", stdout)
			command := regexp.MustCompile(`Next: (.+)$`).FindStringSubmatch(skipped.Message)
			Expect(command).NotTo(BeNil(), "the hint prints the commands: %s", skipped.Message)

			pasteIn(subdirectory, command[1], shell)

			Expect(gitOutputIn(repo, "ls-files")).To(ContainSubstring(".lousy-agents/coach.json"))
			Expect(gitOutputIn(repo, "ls-files")).NotTo(MatchRegexp(`(?m)^project\.json$`))
			Expect(filepath.Join(subdirectory, ".lousy-agents")).NotTo(BeADirectory(), "the command left a stray directory in the subdirectory")
		},
		Entry("under sh", "sh"),
		Entry("under zsh", "zsh"),
	)
})
