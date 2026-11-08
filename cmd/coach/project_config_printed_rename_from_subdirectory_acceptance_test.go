package main

import (
	"os/exec"
	"path/filepath"
	"regexp"

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

// The paths Coach names are repository-relative, as --project-config is, and the rename is described in words, so a
// subdirectory changes nothing about the advice: it is the same message from src/ as from the root.
var _ = Describe("coach codesignal: the rename a refusal describes names repository-relative paths from a subdirectory", func() {
	It("describes a policy committed in the wrong case the same from src/ as from the root", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, "src/keep.go", minimalGoSource)
		commitFile(repo, ".Lousy-Agents/Coach.json", singleRootPolicyJSON)
		scan := func(directory string) string {
			stdout, stderr, exitCode := runCoachBinary(commandPath, directory, nil, "codesignal", "--baseline", "--format=json", "--no-interactive")
			Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
			return string(stderr)
		}

		fromSubdirectory := scan(filepath.Join(repo, "src"))

		Expect(fromSubdirectory).To(Equal(scan(repo)))
		Expect(fromSubdirectory).To(ContainSubstring(makeGitRecordIt(".lousy-agents/coach.json")))
		expectNoRunnableCommand(fromSubdirectory)
	})

	It("describes a named policy committed in the wrong case the same from src/ as from the root", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, "src/keep.go", minimalGoSource)
		commitFile(repo, "policies/p.json", singleRootPolicyJSON)
		scan := func(directory string) string {
			stdout, stderr, exitCode := runCoachBinary(commandPath, directory, nil, "codesignal", "--baseline", "--project-config", "policies/P.json", "--format=json", "--no-interactive")
			Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
			return string(stderr)
		}

		fromSubdirectory := scan(filepath.Join(repo, "src"))

		Expect(fromSubdirectory).To(Equal(scan(repo)))
		Expect(fromSubdirectory).To(ContainSubstring(makeGitRecordIt("policies/P.json")))
		expectNoRunnableCommand(fromSubdirectory)
	})
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
