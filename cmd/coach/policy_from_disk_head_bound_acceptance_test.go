package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// aGitThatFloodsStderrFor is the real git, except that the one subcommand that names the policy's path writes more to
// stderr than Coach's budget for it keeps, and fails. The time bound is 30 seconds in the code and cannot be tripped
// by a spec that is not slow, so the stderr bound stands for the bounds of git at the process boundary.
func aGitThatFloodsStderrFor(subcommand string) []string {
	GinkgoHelper()
	real, err := exec.LookPath("git")
	Expect(err).NotTo(HaveOccurred())
	directory := GinkgoT().TempDir()
	script := fmt.Sprintf("#!/bin/sh\nsub=; target=\nfor a in \"$@\"; do\n\tcase \"$a\" in\n\t\tshow|ls-tree) [ -z \"$sub\" ] && sub=$a;;\n\t\t*%s) target=1;;\n\tesac\ndone\n"+
		"if [ \"$sub\" = %s ] && [ -n \"$target\" ]; then\n\thead -c 70000 /dev/zero | tr '\\0' x >&2\n\texit 128\nfi\nexec %s \"$@\"\n", coachConfigPath, subcommand, real)
	Expect(os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0o755)).To(Succeed())
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "PATH=") {
			env = append(env, entry)
		}
	}
	return append(env, "PATH="+directory+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func runModeWithGit(mode scopePolicyMode, repo, base string, env []string, extra ...string) (stdout, stderr []byte, exitCode int) {
	args := []string{"codesignal", "--baseline"}
	if mode.diff {
		args = []string{"codesignal", "--base", base}
	}
	return runCoachBinary(commandPath, repo, env, append(args, extra...)...)
}

// ADR-009 decision 6: a bound of git that stops the read of HEAD's one copy of the policy leaves that copy unread, as a
// copy over the size budget is, and the usable policy on disk is used. A bound that stops any other read is no such
// disclosure: the repository cannot be read, and the scan fails.
var _ = Describe("coach codesignal reads the project policy from disk: a bound of git that stops the read of HEAD's copy (#168 A2b)", func() {
	boundedModes := []scopePolicyMode{scopePolicyModes[0], scopePolicyModes[2]}

	for _, mode := range boundedModes {
		It("#168 A2b: git's stderr bound tripping while reading the committed blob uses the valid policy on disk with the cause head_unreadable under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, `{"schema_version":"1","roots":["."],"note":"the committed copy"}`})
			scopePolicyRewrite(repo, coachConfigPath, goLayerPolicyConfigJSON)
			env := aGitThatFloodsStderrFor("show")

			stdout, stderr, exitCode := runModeWithGit(mode, repo, base, env, "--format=json")

			Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(scopePolicyScopeOf(stdout)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(coachConfigPath, goLayerPolicyConfigJSON, diskCauseHeadUnreadable))))
		})

		It("#168 A2b: git's stderr bound tripping while reading the committed blob names the bound in the warning and qualifies the all-clear under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, `{"schema_version":"1","roots":["."],"note":"the committed copy"}`})
			scopePolicyRewrite(repo, coachConfigPath, diskPolicyWithoutForbiddenImports)
			env := aGitThatFloodsStderrFor("show")

			text, stderr, exitCode := runModeWithGit(mode, repo, base, env)

			Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", text, stderr)
			after := diskPolicyLinesAfterSummary(mode, text, 2)
			Expect(after).To(HaveLen(2), "text:\n%s", text)
			Expect(after[1]).To(MatchRegexp(`^WARNING: .*HEAD.*could not be read within git's stderr bound`), "text:\n%s", text)
			Expect(after[1]).To(ContainSubstring(coachConfigPath))
			Expect(string(text)).To(MatchRegexp(`(?m)^No active CodeSignal findings, but .*HEAD.*\.$`))
		})

		It("#168 A2b: a bound of git tripping on the listing of the policy's path, not on its blob, fails the scan under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, `{"schema_version":"1","roots":["."],"note":"the committed copy"}`})
			scopePolicyRewrite(repo, coachConfigPath, goLayerPolicyConfigJSON)
			env := aGitThatFloodsStderrFor("ls-tree")

			stdout, stderr, exitCode := runModeWithGit(mode, repo, base, env, "--format=json")

			Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stdout)).To(BeEmpty())
			Expect(string(stderr)).NotTo(ContainSubstring("head_unreadable"))
		})
	}

	// The time bound is a 30-second constant of the code, so the port that reads HEAD's copy answers for it.
	It("#168 A2b: git's time bound tripping while reading the committed blob names the bound in the warning and keeps the cause head_unreadable", func() {
		repo, _ := diskPolicyRepo(scopePolicyFile{coachConfigPath, `{"schema_version":"1","roots":["."],"note":"the committed copy"}`})
		scopePolicyRewrite(repo, coachConfigPath, goLayerPolicyConfigJSON)
		ports := defaultScanPorts()
		ports.readCommitted = func(string, string, string) (projectconfig.CommittedCopy, error) {
			return projectconfig.CommittedCopy{Kind: projectconfig.CopyUnreadable, Bound: projectconfig.CopyBoundTime}, nil
		}
		original, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chdir(repo)).To(Succeed())
		defer func() { Expect(os.Chdir(original)).To(Succeed()) }()

		text, stderr, exitCode := runInProcessWith(ports, "codesignal", "--baseline")
		stdout, _, _ := runInProcessWith(ports, "codesignal", "--baseline", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(scopePolicyScopeOf(stdout)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(coachConfigPath, goLayerPolicyConfigJSON, diskCauseHeadUnreadable))))
		after := diskPolicyLinesAfterSummary(scopePolicyModes[0], text, 2)
		Expect(after).To(HaveLen(2), "text:\n%s", text)
		Expect(after[1]).To(MatchRegexp(`^WARNING: .*HEAD.*could not be read within git's time bound`), "text:\n%s", text)
	})
})
