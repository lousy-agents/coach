package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// gitLayout is what git reports about the repository a directory belongs to. The guard specs assert it as a
// fixture precondition, so a spec cannot pass because its fixture fell outside the layout it names.
type gitLayout struct {
	gitDir, commonDir, topLevel, coreWorktree string
}

func layoutOf(dir string, extra []string) gitLayout {
	return gitLayout{
		gitDir:       gitWith(dir, extra, "rev-parse", "--absolute-git-dir"),
		commonDir:    gitWith(dir, extra, "rev-parse", "--path-format=absolute", "--git-common-dir"),
		topLevel:     gitWith(dir, extra, "rev-parse", "--show-toplevel"),
		coreWorktree: gitWith(dir, extra, "config", "--default", "", "--get", "core.worktree"),
	}
}

func codesignalJSONIn(dir string, extra []string, args ...string) (stdout, stderr []byte, exitCode int) {
	return runCoachBinary(commandPath, dir, append(os.Environ(), extra...), append([]string{"codesignal", "--format=json"}, args...)...)
}

var _ = Describe("coach codesignal: a scan reads a policy from a git layout git itself links to its working tree (#168 A2a)", func() {
	expectScansWithDiskPolicy := func(dir string) {
		stdout, stderr, exitCode := codesignalJSONIn(dir, nil, "--baseline", "--project-config", explicitPolicyPath)

		ExpectWithOffset(1, exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
		ExpectWithOffset(1, scopePolicyScopeOf(stdout)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(explicitPolicyPath, singleRootPolicyJSON, true))))
	}

	It("scans a linked worktree, whose git directory is not its repository's main one", func() {
		repo := commitExplicitPolicyRepo()
		linked := filepath.Join(GinkgoT().TempDir(), "linked")
		runGitInRepo(repo, "worktree", "add", "-q", "--detach", linked)
		layout := layoutOf(linked, nil)
		Expect(layout.gitDir).NotTo(Equal(layout.commonDir), "the fixture must be a linked worktree")
		Expect(layout.coreWorktree).To(BeEmpty(), "core.worktree must not be what links this layout")
		Expect(filepath.Dir(layout.gitDir)).NotTo(Equal(layout.topLevel), "the git directory's parent must not be what links this layout")
		Expect(filepath.Join(linked, explicitPolicyPath)).To(BeAnExistingFile())

		expectScansWithDiskPolicy(linked)
	})

	It("scans a submodule's work tree, whose git directory sits under the superproject and names it by core.worktree", func() {
		repo := commitScannableGoRepo()
		inner := commitExplicitPolicyRepo()
		runGitInRepo(repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", inner, "sub")
		submodule := filepath.Join(repo, "sub")
		layout := layoutOf(submodule, nil)
		Expect(layout.coreWorktree).NotTo(BeEmpty(), "the fixture must be a submodule whose git directory names its work tree")
		Expect(layout.gitDir).To(Equal(layout.commonDir), "the fixture must not be a linked worktree")
		Expect(filepath.Dir(layout.gitDir)).NotTo(Equal(layout.topLevel), "the git directory's parent must not be what links this layout")
		Expect(layout.topLevel).To(Equal(submodule))
		Expect(filepath.Join(submodule, ".git")).To(BeARegularFile())

		expectScansWithDiskPolicy(submodule)
	})

	It("scans a plain repository from a subdirectory, where the git directory's parent is the repository's top level", func() {
		repo := commitExplicitPolicyRepo()
		subdirectory := filepath.Join(repo, "policies")
		layout := layoutOf(subdirectory, nil)
		Expect(layout.gitDir).To(Equal(layout.commonDir))
		Expect(layout.coreWorktree).To(BeEmpty())
		Expect(filepath.Dir(layout.gitDir)).To(Equal(layout.topLevel))
		Expect(layout.topLevel).NotTo(Equal(subdirectory), "the scan must start below the top level")

		expectScansWithDiskPolicy(subdirectory)
	})
})

var _ = Describe("coach codesignal: a scan fails closed when git's repository is not the one that holds the working tree (#168 A2a)", func() {
	viaEnvironment := func(_, other string) []string {
		return []string{"GIT_DIR=" + filepath.Join(other, ".git")}
	}
	viaGitFile := func(repo, other string) []string {
		Expect(os.RemoveAll(filepath.Join(repo, ".git"))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: "+filepath.Join(other, ".git")+"\n"), 0o644)).To(Succeed())
		return nil
	}

	DescribeTable("exits 2 with no report, naming the git directory, whether or not a policy is read",
		func(withPolicy bool, relocate func(repo, other string) []string, scanArgs ...string) {
			repo := commitScannableGoRepo()
			if withPolicy {
				repo = commitExplicitPolicyRepo()
				scanArgs = append(scanArgs, "--project-config", explicitPolicyPath)
			}
			other := commitScannableGoRepo()
			extra := relocate(repo, other)
			layout := layoutOf(repo, extra)
			Expect(layout.gitDir).To(Equal(filepath.Join(other, ".git")), "the fixture must point git at the other repository")
			Expect(layout.topLevel).To(Equal(repo), "the fixture must scan from the first repository's working tree")
			Expect(layout.gitDir).To(Equal(layout.commonDir), "the fixture must name a main git directory")
			Expect(layout.coreWorktree).To(BeEmpty())
			Expect(filepath.Dir(layout.gitDir)).NotTo(Equal(layout.topLevel))

			stdout, stderr, exitCode := codesignalJSONIn(repo, extra, scanArgs...)

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring("is not the one that holds this working tree"))
			Expect(string(stderr)).To(ContainSubstring(layout.gitDir))
			Expect(stdout).To(BeEmpty())
		},
		Entry("GIT_DIR names another repository, and a policy is on disk", true, viaEnvironment, "--baseline"),
		Entry("a .git file names another repository, and a policy is on disk", true, viaGitFile, "--baseline"),
		Entry("GIT_DIR names another repository, and there is no policy", false, viaEnvironment, "--baseline"),
		Entry("a .git file names another repository, and there is no policy", false, viaGitFile, "--baseline"),
		Entry("GIT_DIR names another repository, and the scan is a diff", false, viaEnvironment, "--base", "HEAD"),
	)
})

// The two layouts the fail-closed rule must not refuse: each flips one clause of the rule on its own, so a rule
// that compared git's top level with the git directory's parent, or that ignored GIT_WORK_TREE, fails one of them.
var _ = Describe("coach codesignal: a scan still reads a repository whose git directory the caller names (#168 A2a)", func() {
	It("scans from a subdirectory when GIT_DIR names the repository's own git directory, which makes git take that subdirectory for the top level", func() {
		repo := commitExplicitPolicyRepo()
		subdirectory := filepath.Join(repo, "policies")
		extra := []string{"GIT_DIR=" + filepath.Join(repo, ".git")}
		layout := layoutOf(subdirectory, extra)
		Expect(layout.gitDir).To(Equal(layout.commonDir))
		Expect(layout.coreWorktree).To(BeEmpty())
		Expect(layout.topLevel).To(Equal(subdirectory), "the fixture must make git's top level differ from the git directory's parent")
		Expect(filepath.Dir(layout.gitDir)).To(Equal(repo))
		Expect(filepath.Join(subdirectory, explicitPolicyPath)).NotTo(BeAnExistingFile())

		stdout, stderr, exitCode := codesignalJSONIn(subdirectory, extra, "--baseline")

		Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
	})

	It("scans a work tree whose git directory git init --separate-git-dir put elsewhere (a foreign gitdir with a non-.git name cannot be told apart from this one, so that limit is accepted)", func() {
		repo := commitExplicitPolicyRepo()
		store := filepath.Join(GinkgoT().TempDir(), "store.git")
		runGitInRepo(repo, "init", "-q", "--separate-git-dir="+store, repo)
		layout := layoutOf(repo, nil)
		Expect(layout.coreWorktree).To(BeEmpty(), "core.worktree must not be what links this layout")
		Expect(filepath.Base(layout.gitDir)).NotTo(Equal(".git"), "the fixture must keep its git directory under another name")
		Expect(layout.gitDir).To(Equal(layout.commonDir))
		Expect(layout.topLevel).To(Equal(repo))
		Expect(filepath.Dir(layout.gitDir)).NotTo(Equal(repo))
		Expect(repo).NotTo(HavePrefix(filepath.Dir(layout.gitDir)+string(filepath.Separator)), "the work tree must not sit inside the git directory's parent")
		Expect(filepath.Join(repo, ".git")).To(BeARegularFile())

		stdout, stderr, exitCode := codesignalJSONIn(repo, nil, "--baseline", "--project-config", explicitPolicyPath)

		Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
		Expect(scopePolicyScopeOf(stdout)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(explicitPolicyPath, singleRootPolicyJSON, true))))
	})

	It("scans a work tree whose .git is a symlink to a git directory kept elsewhere under another name", func() {
		repo := commitExplicitPolicyRepo()
		store := filepath.Join(GinkgoT().TempDir(), "stores", "wt.git")
		Expect(os.MkdirAll(filepath.Dir(store), 0o755)).To(Succeed())
		Expect(os.Rename(filepath.Join(repo, ".git"), store)).To(Succeed())
		Expect(os.Symlink(store, filepath.Join(repo, ".git"))).To(Succeed())
		layout := layoutOf(repo, nil)
		Expect(layout.coreWorktree).To(BeEmpty(), "core.worktree must not be what links this layout")
		Expect(layout.gitDir).To(Equal(layout.commonDir))
		Expect(filepath.Base(layout.gitDir)).NotTo(Equal(".git"), "git must report the resolved git directory, not the symlink")
		Expect(layout.topLevel).To(Equal(repo))
		Expect(repo).NotTo(HavePrefix(filepath.Dir(layout.gitDir)+string(filepath.Separator)), "the work tree must not sit inside the git directory's parent")

		stdout, stderr, exitCode := codesignalJSONIn(repo, nil, "--baseline", "--project-config", explicitPolicyPath)

		Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
		Expect(scopePolicyScopeOf(stdout)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(explicitPolicyPath, singleRootPolicyJSON, true))))
	})

	It("scans a work tree that GIT_WORK_TREE names, though its git directory sits in a directory that holds no work tree", func() {
		repo := commitExplicitPolicyRepo()
		gitDirectory := filepath.Join(GinkgoT().TempDir(), "store.git")
		Expect(os.Rename(filepath.Join(repo, ".git"), gitDirectory)).To(Succeed())
		extra := []string{"GIT_DIR=" + gitDirectory, "GIT_WORK_TREE=" + repo}
		layout := layoutOf(repo, extra)
		Expect(layout.gitDir).To(Equal(layout.commonDir))
		Expect(layout.coreWorktree).To(BeEmpty(), "core.worktree must not be what links this layout")
		Expect(layout.topLevel).To(Equal(repo))
		Expect(filepath.Dir(layout.gitDir)).NotTo(Equal(repo))

		stdout, stderr, exitCode := codesignalJSONIn(repo, extra, "--baseline", "--project-config", explicitPolicyPath)

		Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
		Expect(scopePolicyScopeOf(stdout)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(explicitPolicyPath, singleRootPolicyJSON, true))))
	})
})
