package main

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: a hostile git configuration or environment executes nothing", func() {
	script := func(marker string) string {
		path := filepath.Join(GinkgoT().TempDir(), "run.sh")
		Expect(os.WriteFile(path, []byte("#!/bin/sh\ntouch '"+marker+"'\nprintf '\\0'\n"), 0o755)).To(Succeed())
		return path
	}

	It("runs no core.fsmonitor program, from the repository config or from GIT_CONFIG_COUNT", func() {
		marker := filepath.Join(GinkgoT().TempDir(), "ran")
		program := script(marker)
		repo := commitExplicitPolicyRepo()
		gitIn(repo, "config", "core.fsmonitor", program)
		env := append(os.Environ(), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.fsmonitor", "GIT_CONFIG_VALUE_0="+program)

		_, stderr, exitCode := runCoachBinary(commandPath, repo, env, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(os.Remove(filepath.Join(repo, explicitPolicyPath))).To(Succeed())
		refusedViaEnv := func() {
			_, _, code := runCoachBinary(commandPath, repo, env, "codesignal", "--baseline", "--project-config", explicitPolicyPath)
			Expect(code).To(Equal(2))
		}
		refusedViaEnv()

		Expect(marker).NotTo(BeAnExistingFile(), "git ran the core.fsmonitor program the repository's configuration names")
	})

	// Each program has its own marker, so a failure names the one that ran. The textconv driver and the
	// external diff program are not here: git runs them on a patch, and a scan prints one only for a file
	// that changed between the base and HEAD (hostile_diff_driver_acceptance_test.go).
	expectNoProgramRan := func(arrange func(repo, program string, installHook func(name string)) []string, programChannel string) {
		marker := filepath.Join(GinkgoT().TempDir(), "ran")
		program := script(marker)
		repo := commitExplicitPolicyRepo()
		// installed after the setup commit, which would otherwise run them itself
		installHook := func(name string) {
			hook := filepath.Join(repo, ".git", "hooks", name)
			Expect(os.WriteFile(hook, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o755)).To(Succeed())
		}
		// a changed file in the worktree, so a read that is allowed to refresh the index has a reason to
		Expect(os.WriteFile(filepath.Join(repo, "a.go"), []byte(minimalGoSource+"// changed\n"), 0o644)).To(Succeed())
		env := append(os.Environ(), arrange(repo, program, installHook)...)

		for _, args := range [][]string{
			{"codesignal", "--base", "HEAD", "--project-config", explicitPolicyPath, "--format=json"},
			{"codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json"},
		} {
			_, stderr, exitCode := runCoachBinary(commandPath, repo, env, args...)
			Expect(exitCode).To(BeNumerically("<=", 1), "stderr: %s", stderr)
		}

		Expect(marker).NotTo(BeAnExistingFile(), "git ran a program named by "+programChannel)
	}

	// The one hook a read-only scan can trigger: git runs it when a read rewrites the index. Dropping
	// --no-optional-locks from the git arguments makes this entry fail.
	DescribeTable("runs no hook that a read-only scan can trigger",
		expectNoProgramRan,
		Entry("the post-index-change hook, which git runs when a read rewrites the index", func(_, _ string, installHook func(string)) []string {
			installHook("post-index-change")
			return nil
		}, "the post-index-change hook"),
	)

	// No read Coach runs updates a ref, checks out, merges, starts a pager, asks for a credential, opens an
	// ssh connection or opens an editor, and the harness has no terminal and no remote, so none of these
	// entries can fail today. They are tripwires for a read that would, each naming its program, and they
	// pin nothing else. (The ssh channel is pinned where it can fail: the partial-clone specs, whose
	// lazy fetch would run it.)
	DescribeTable("names the hooks and environment programs that no read Coach runs can reach: these entries cannot fail today and pin nothing but the names",
		expectNoProgramRan,
		Entry("the post-checkout hook", func(_, _ string, installHook func(string)) []string {
			installHook("post-checkout")
			return nil
		}, "the post-checkout hook"),
		Entry("the reference-transaction hook", func(_, _ string, installHook func(string)) []string {
			installHook("reference-transaction")
			return nil
		}, "the reference-transaction hook"),
		Entry("the post-merge hook", func(_, _ string, installHook func(string)) []string {
			installHook("post-merge")
			return nil
		}, "the post-merge hook"),
		Entry("GIT_PAGER", func(_, program string, _ func(string)) []string { return []string{"GIT_PAGER=" + program} }, "GIT_PAGER"),
		Entry("PAGER", func(_, program string, _ func(string)) []string { return []string{"PAGER=" + program} }, "PAGER"),
		Entry("GIT_ASKPASS", func(_, program string, _ func(string)) []string { return []string{"GIT_ASKPASS=" + program} }, "GIT_ASKPASS"),
		Entry("GIT_SSH_COMMAND", func(_, program string, _ func(string)) []string { return []string{"GIT_SSH_COMMAND=" + program} }, "GIT_SSH_COMMAND"),
		Entry("GIT_EDITOR", func(_, program string, _ func(string)) []string { return []string{"GIT_EDITOR=" + program} }, "GIT_EDITOR"),
	)

	// The variables that point git at another work tree, index or repository are removed before git starts, so
	// a scan reports on the repository it was run in whatever its environment says. Honoring them would have
	// it read one repository's history against another's working tree.
	It("ignores GIT_WORK_TREE, GIT_INDEX_FILE and GIT_DIR, and reports on the repository it was run in", func() {
		repo := commitExplicitPolicyRepo()
		bare := repo + ".bare"
		Expect(exec.Command("git", "clone", "-q", "--bare", repo, bare).Run()).To(Succeed())
		DeferCleanup(func() { _ = os.RemoveAll(bare) })
		args := []string{"codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json"}

		wantOut, wantErr, wantExit := runCoachBinary(commandPath, repo, os.Environ(), args...)
		pointedElsewhere, errElsewhere, exitElsewhere := runCoachBinary(commandPath, repo, append(os.Environ(), "GIT_WORK_TREE="+GinkgoT().TempDir(), "GIT_INDEX_FILE=/nonexistent/index"), args...)
		viaBare, errBare, exitBare := runCoachBinary(commandPath, filepath.Join(repo, "policies"), append(os.Environ(), "GIT_DIR="+bare), args...)
		fromSubdirectory, _, exitSubdirectory := runCoachBinary(commandPath, filepath.Join(repo, "policies"), os.Environ(), args...)

		Expect(exitElsewhere).To(Equal(wantExit), "stderr: %s", errElsewhere)
		Expect(pointedElsewhere).To(Equal(wantOut), "a different work tree or index changed the report")
		Expect(string(errElsewhere)).To(Equal(string(wantErr)))
		Expect(exitBare).To(Equal(exitSubdirectory), "stderr: %s", errBare)
		Expect(viaBare).To(Equal(fromSubdirectory), "a bare repository named by GIT_DIR changed the report")
	})

	It("ignores GIT_DIR naming a different repository, and fails closed when a .git file points at one", func() {
		repo := commitExplicitPolicyRepo()
		other := commitScannableGoRepo()
		args := []string{"codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json"}

		wantOut, _, wantExit := runCoachBinary(commandPath, repo, os.Environ(), args...)
		viaEnvOut, stderr, viaEnv := runCoachBinary(commandPath, repo, append(os.Environ(), "GIT_DIR="+filepath.Join(other, ".git")), args...)
		Expect(viaEnv).To(Equal(wantExit), "stderr: %s", stderr)
		Expect(viaEnvOut).To(Equal(wantOut), "a scan read one repository's history against another's working tree")

		Expect(os.RemoveAll(filepath.Join(repo, ".git"))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: "+filepath.Join(other, ".git")+"\n"), 0o644)).To(Succeed())
		stdout, stderr, viaFile := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(viaFile).To(Equal(2), "stderr: %s", stderr)
		Expect(string(stderr)).To(ContainSubstring("exists in the worktree but is not committed"))
		Expect(stdout).To(BeEmpty())
	})
})
