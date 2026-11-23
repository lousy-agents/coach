package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

var _ = Describe("coach codesignal --prepare-compiler: what the refusal says about a policy path that holds something other than a usable file", func() {
	const policy = "pol.json"

	It("says to fix the file, once, and offers no authoring, when a file on disk cannot be read", func() {
		if os.Geteuid() == 0 {
			Skip("a file mode does not keep root out; `mise run acceptance-ci-env` runs it as non-root")
		}
		repo := typescriptRepoWithoutPolicy()
		writeWorktreeFile(repo, policy, singleRootPolicyJSON)
		Expect(os.Chmod(filepath.Join(repo, policy), 0o000)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(filepath.Join(repo, policy), 0o644) })

		run := prepareCompilerAnswering(repo, policyref.Named(policy), "")

		Expect(run.exitCode).To(Equal(2), "transcript:\n%s", run.transcript)
		Expect(run.transcript).To(ContainSubstring("a project policy that passes --check-project "+refusalWords+" (policy_invalid)"), "transcript:\n%s", run.transcript)
		Expect(strings.Count(run.transcript, unusablePolicyStep)).To(Equal(1), "transcript:\n%s", run.transcript)
		Expect(run.transcript).NotTo(ContainSubstring("author_policy"), "transcript:\n%s", run.transcript)
		Expect(run.transcript).NotTo(ContainSubstring("resolve the policy path first"), "transcript:\n%s", run.transcript)
	})

	DescribeTable("still says to resolve the policy path first, and not that the file is to be fixed",
		func(arrange func(repo string)) {
			repo := typescriptRepoWithoutPolicy()
			arrange(repo)

			run := prepareCompilerAnswering(repo, policyref.Named(policy), "")

			Expect(run.exitCode).To(Equal(2), "transcript:\n%s", run.transcript)
			Expect(run.transcript).To(ContainSubstring("a project policy that passes --check-project "+refusalWords+" (policy_invalid); a new policy cannot fix it, so resolve the policy path first."), "transcript:\n%s", run.transcript)
			Expect(run.transcript).NotTo(ContainSubstring(unusablePolicyStep), "transcript:\n%s", run.transcript)
			Expect(run.transcript).NotTo(ContainSubstring("author_policy"), "transcript:\n%s", run.transcript)
		},
		Entry("a directory on disk", func(repo string) {
			Expect(os.Mkdir(filepath.Join(repo, policy), 0o755)).To(Succeed())
		}),
		Entry("a symbolic link on disk", func(repo string) {
			Expect(os.Symlink("/etc/hostname", filepath.Join(repo, policy))).To(Succeed())
		}),
	)
})
