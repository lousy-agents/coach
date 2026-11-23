package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

const (
	droppedPolicyCauseWords = `\b(not_committed|staged_edit|changed_on_disk|staged_copy|staged_not_a_file|merge_conflict|committed_as_link|index_locked|git_unreadable)\b`
	installPrompt           = "Type 'install' to run this setup command now"
	refusalWords            = "is required before compiler setup"
)

type preparedCompiler struct {
	exitCode   int
	transcript string
}

func prepareCompilerAnswering(repo string, policy policyref.Ref, answers string) preparedCompiler {
	GinkgoHelper()
	writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \""+tstoolchain.SupportedTypescriptVersions[0]+"\"\n")
	miseDir := writeStatefulStubMiseScript(tstoolchain.SupportedTypescriptVersions[0])
	GinkgoT().Setenv("PATH", writeStubNodeScript("v24.9.9")+string(os.PathListSeparator)+miseDir+string(os.PathListSeparator)+pathExcludingToolchain())
	GinkgoT().Setenv("HOME", os.Getenv("HOME"))
	stdin := authoringStdin(answers)
	DeferCleanup(stdin.Close)
	stdoutFile, stderrFile, _, readStderr := authoringOutputFiles()
	DeferCleanup(stdoutFile.Close)
	DeferCleanup(stderrFile.Close)

	exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, policy)

	return preparedCompiler{exitCode: exitCode, transcript: readStderr()}
}

func typescriptRepoWithoutPolicy() string {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
	return repo
}

var _ = Describe("coach codesignal --prepare-compiler with a policy that is valid on disk", func() {
	const policy = "pol.json"

	type onDisk struct {
		ref     policyref.Ref
		arrange func(repo string)
		warning string
	}

	DescribeTable("offers compiler setup, and says before the install prompt how the policy differs from the committed one",
		func(s onDisk) {
			repo := typescriptRepoWithoutPolicy()
			s.arrange(repo)

			run := prepareCompilerAnswering(repo, s.ref, "mise_project\ndecline\n")

			Expect(run.transcript).NotTo(ContainSubstring(refusalWords), "transcript:\n%s", run.transcript)
			Expect(run.transcript).To(ContainSubstring(installPrompt), "the offer must reach its install prompt; transcript:\n%s", run.transcript)
			Expect(run.transcript).To(ContainSubstring(s.warning), "transcript:\n%s", run.transcript)
			Expect(strings.Index(run.transcript, s.warning)).To(BeNumerically("<", strings.Index(run.transcript, installPrompt)), "transcript:\n%s", run.transcript)
		},
		Entry("a policy that is not committed or staged", onDisk{policyref.Named(policy), func(repo string) {
			writeWorktreeFile(repo, policy, singleRootPolicyJSON)
		}, "project_policy_not_committed"}),
		Entry("a policy that is staged and never committed", onDisk{policyref.Named(policy), func(repo string) {
			writeWorktreeFile(repo, policy, singleRootPolicyJSON)
			gitIn(repo, "add", policy)
		}, "project_policy_not_committed"}),
		Entry("a committed policy edited on disk", onDisk{policyref.Named(policy), func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			writeWorktreeFile(repo, policy, validPolicyWithLayers)
		}, "project_policy_differs_from_head"}),
		Entry("a committed policy edited on disk and staged", onDisk{policyref.Named(policy), func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			writeWorktreeFile(repo, policy, validPolicyWithLayers)
			gitIn(repo, "add", policy)
		}, "project_policy_differs_from_head"}),
		Entry("a policy whose index entry is conflicted", onDisk{policyref.Named(policy), func(repo string) {
			stageThreeSidesOf(repo, policy)
			writeWorktreeFile(repo, policy, singleRootPolicyJSON)
		}, "project_policy_differs_from_head"}),
		Entry("a policy committed as a symbolic link", onDisk{policyref.Named(policy), func(repo string) {
			commitLinkThenWriteFile(repo, policy)
		}, "project_policy_differs_from_head"}),
		Entry("a committed policy whose blob is gone from the repository", onDisk{policyref.Named(policy), func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			blob := strings.TrimSpace(gitIn(repo, "rev-parse", "HEAD:"+policy))
			Expect(os.Remove(filepath.Join(repo, ".git", "objects", blob[:2], blob[2:]))).To(Succeed())
		}, "project_policy_head_unreadable"}),
		Entry("a committed policy while another git process holds the index", onDisk{policyref.Named(policy), func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			Expect(os.WriteFile(filepath.Join(repo, ".git", "index.lock"), nil, 0o644)).To(Succeed())
		}, "project_policy_head_unreadable"}),
		Entry("a discovered policy that is not committed", onDisk{policyref.Discovered(""), func(repo string) {
			writeWorktreeFile(repo, coachConfigPath, singleRootPolicyJSON)
		}, "project_policy_not_committed"}),
		Entry("a discovered policy whose committed copy is invalid, with a valid fix on disk", onDisk{policyref.Discovered(""), func(repo string) {
			commitFile(repo, coachConfigPath, invalidPolicy)
			writeWorktreeFile(repo, coachConfigPath, validPolicyWithLayers)
		}, "project_policy_differs_from_head"}),
	)
})

var _ = Describe("coach codesignal --prepare-compiler with a policy that cannot be used", func() {
	const policy = "pol.json"

	DescribeTable("refuses an invalid file on disk with no new policy to author, whatever the committed copy is",
		func(arrange func(repo string)) {
			repo := typescriptRepoWithoutPolicy()
			arrange(repo)

			run := prepareCompilerAnswering(repo, policyref.Named(policy), "")

			Expect(run.exitCode).To(Equal(2), "transcript:\n%s", run.transcript)
			Expect(run.transcript).NotTo(ContainSubstring("author_policy"), "authoring refuses a path that exists; transcript:\n%s", run.transcript)
			Expect(run.transcript).To(ContainSubstring("a new policy cannot fix it"), "transcript:\n%s", run.transcript)
			Expect(run.transcript).To(ContainSubstring("a project policy that passes --check-project "+refusalWords+" (policy_invalid)"), "transcript:\n%s", run.transcript)
			Expect(run.transcript).NotTo(ContainSubstring("guided policy authoring"), "transcript:\n%s", run.transcript)
			Expect(run.transcript).NotTo(ContainSubstring("move it aside"), "transcript:\n%s", run.transcript)
			Expect(run.transcript).NotTo(MatchRegexp(droppedPolicyCauseWords), "transcript:\n%s", run.transcript)
		},
		Entry("an invalid document committed and on disk", func(repo string) {
			commitFile(repo, policy, invalidPolicy)
		}),
		Entry("an invalid document that is not committed", func(repo string) {
			writeWorktreeFile(repo, policy, invalidPolicy)
		}),
		Entry("an invalid fix on disk over a valid committed policy", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			writeWorktreeFile(repo, policy, invalidPolicy)
		}),
	)

	It("offers neither a restore from the revision nor a dropped cause word for a staged edit whose file is deleted on disk", func() {
		repo := typescriptRepoWithoutPolicy()
		commitFile(repo, policy, singleRootPolicyJSON)
		writeWorktreeFile(repo, policy, validPolicyWithLayers)
		gitIn(repo, "add", policy)
		Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())

		run := prepareCompilerAnswering(repo, policyref.Named(policy), "")

		Expect(run.exitCode).To(Equal(2), "transcript:\n%s", run.transcript)
		Expect(run.transcript).To(ContainSubstring("a project policy that passes --check-project "+refusalWords+" (policy_missing)"), "transcript:\n%s", run.transcript)
		Expect(run.transcript).NotTo(ContainSubstring("--source="), "transcript:\n%s", run.transcript)
		Expect(run.transcript).NotTo(ContainSubstring("--staged --worktree"), "transcript:\n%s", run.transcript)
		Expect(run.transcript).NotTo(MatchRegexp(droppedPolicyCauseWords), "transcript:\n%s", run.transcript)
	})
})
