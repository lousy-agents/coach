package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

// writeInstallingSetupExecutable writes an executable named `name` that
// models a real npm/pnpm/bun install succeeding: it writes a real-shaped
// node_modules/typescript (plus its matching native platform package) into
// its own working directory, mirroring writeInstalledTypescriptUnder's
// fixture shape, then exits 0. This is what actually flips
// checks.compiler from typescript_compiler_missing to pass across a
// readiness recheck -- resolveCompiler reads exactly these two files.
func writeInstallingSetupExecutable(name, version string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-setup-recheck-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	nativeName := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
	tsManifest := fmt.Sprintf(`{"name":"typescript","version":"%s"}`, version)
	nativeManifest := fmt.Sprintf(`{"name":"@typescript/%s","version":"%s"}`, nativeName, version)

	script := "#!/bin/sh\n" +
		"mkdir -p node_modules/typescript\n" +
		"cat > node_modules/typescript/package.json <<'EOF'\n" + tsManifest + "\nEOF\n" +
		"mkdir -p node_modules/@typescript/" + nativeName + "\n" +
		"cat > node_modules/@typescript/" + nativeName + "/package.json <<'EOF'\n" + nativeManifest + "\nEOF\n" +
		"exit 0\n"

	Expect(os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755)).To(Succeed())
	return dir
}

// setupRecheckSupportedTypescriptVersion is the exact compiler version this
// build accepts, read from the same exported set CheckProjectReadiness
// itself resolves against -- so the fixture below cannot silently drift
// from the compiler this build actually supports.
func setupRecheckSupportedTypescriptVersion() string {
	versions := codesignalcli.SupportedTypescriptVersions
	Expect(versions).NotTo(BeEmpty())
	return versions[len(versions)-1]
}

// setupRecheckNativeCompilerPackagePath is the dirty-worktree path of the
// native platform package writeInstallingSetupExecutable writes, computed
// from this test environment's own GOOS/arch rather than a literal, since
// the native package name is architecture-specific.
func setupRecheckNativeCompilerPackagePath() string {
	return fmt.Sprintf("node_modules/@typescript/typescript-%s-%s/package.json", runtime.GOOS, npmArchName())
}

var _ = Describe("codesignalcli.RunConfirmedSetupAndRecheckReadiness (AC-SET-6)", func() {
	When("a confirmed project-package install succeeds and resolves the previously missing compiler", func() {
		It("reruns the complete readiness check in-process, replacing the stale pre-install compiler gap with a pass", func() {
			GinkgoT().Setenv("PATH", pathWithStubNode("v24.9.9"))

			repo := newTempGitRepo()
			version := setupRecheckSupportedTypescriptVersion()
			commitFile(repo, "package.json", fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":%q}}`+"\n", version))
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			// A non-default config path: this is deliberate, so a caller that
			// silently drops or defaults configPath (rather than threading it
			// through to the post-install recheck) produces a policy_missing
			// failure here instead of an accidental pass.
			commitFile(repo, "coach-project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			head := commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			before, err := codesignalcli.CheckProjectReadiness(repo, head, "coach-project.json")
			Expect(err).NotTo(HaveOccurred())
			Expect(before.Checks.Compiler.State).To(Equal(codesignalcli.ReadinessFail))
			Expect(before.Checks.Compiler.Code).To(Equal(codesignalcli.GapTypescriptCompilerMissing), "the fixture must genuinely start with a missing-compiler gap, or a later pass proves nothing")
			Expect(before.Status).To(Equal(codesignalcli.StatusNeedsPrerequisite), "the pre-install snapshot must genuinely be blocked, or the post-install comparison proves nothing")

			stubDir := writeInstallingSetupExecutable("npm", version)
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+pathWithStubNode("v24.9.9"))

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", repo)
			Expect(err).NotTo(HaveOccurred())

			outcome, err := codesignalcli.RunConfirmedSetupAndRecheckReadiness(context.Background(), preview, true, repo, head, "coach-project.json")
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome.Kind).To(Equal(codesignalcli.SetupOutcomeSucceeded))

			Expect(outcome.PostInstallReadiness).NotTo(BeNil(), "a successful install must trigger a real, in-process post-install readiness recheck (AC-SET-6)")
			post := outcome.PostInstallReadiness

			// Fields only a genuine, complete CheckProjectReadiness of this
			// repo/revision can produce -- not merely a compiler-only partial
			// recheck, and not a value hard-coded ahead of time. Asserting the
			// runtime/package-manager/policy legs kills a partial-recheck
			// mutant; asserting DirtyWorktree.Paths (which exist only because
			// the stub install just wrote them) proves the second read hit the
			// post-install filesystem rather than returning a canned result.
			Expect(post.SchemaVersion).To(Equal(codesignalcli.ReadinessSchemaVersion))
			Expect(post.Revision).To(Equal(head))
			Expect(post.Status).To(Equal(codesignalcli.StatusReadyWithLimits), "must have advanced past the pre-install needs_prerequisite status")

			Expect(post.Checks.Compiler.State).To(Equal(codesignalcli.ReadinessPass), "the recheck must observe the just-installed compiler, not the stale pre-install snapshot")
			Expect(post.Checks.Compiler.Version).To(Equal(version))

			Expect(post.Checks.Runtime.State).To(Equal(codesignalcli.ReadinessPass))
			Expect(post.Checks.Runtime.Version).To(Equal("v24.9.9"))
			Expect(post.Checks.Runtime.Origin).To(Equal("path"))
			Expect(post.Checks.Node.Version).To(Equal("v24.9.9"))

			Expect(post.Checks.Policy.State).To(Equal(codesignalcli.ReadinessPass), "a dropped/defaulted configPath would report policy_missing here instead")
			Expect(post.Checks.ProjectShape.State).To(Equal(codesignalcli.ReadinessPass))

			Expect(post.Checks.PackageManager.State).To(Equal(codesignalcli.ReadinessFail))
			Expect(post.Checks.PackageManager.Code).To(Equal(codesignalcli.GapPackageManagerVersionUnverifiable))
			Expect(post.Checks.PackageManager.Kind).To(Equal("npm"))

			Expect(post.DirtyWorktree.Paths).To(ContainElement("node_modules/typescript/package.json"), "these files exist only because the stub install just wrote them post-recheck")
			Expect(post.DirtyWorktree.Paths).To(ContainElement(setupRecheckNativeCompilerPackagePath()))
		})
	})

	When("the setup is cancelled", func() {
		It("does not rerun readiness, since nothing was installed to recheck", func() {
			GinkgoT().Setenv("PATH", pathWithStubNode("v24.9.9"))

			repo := newTempGitRepo()
			version := setupRecheckSupportedTypescriptVersion()
			commitFile(repo, "package.json", fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":%q}}`+"\n", version))
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			head := commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			stubDir := writeInstallingSetupExecutable("npm", version)
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+pathWithStubNode("v24.9.9"))

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", repo)
			Expect(err).NotTo(HaveOccurred())

			outcome, err := codesignalcli.RunConfirmedSetupAndRecheckReadiness(context.Background(), preview, false, repo, head, "project.json")
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome.Kind).To(Equal(codesignalcli.SetupOutcomeCancelled))
			Expect(outcome.PostInstallReadiness).To(BeNil(), "a cancelled setup must never rerun readiness")

			_, statErr := os.Stat(filepath.Join(repo, "node_modules"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "cancellation must never have run the installing stub")
		})
	})

	When("the confirmed install itself fails", func() {
		It("reports SetupOutcomeFailed without ever attempting a recheck, since a failed install per T5 never proceeds to recheck", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			head := commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			stubDir := writeFailingSetupExecutableWithResidue("npm")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", repo)
			Expect(err).NotTo(HaveOccurred())

			outcome, err := codesignalcli.RunConfirmedSetupAndRecheckReadiness(context.Background(), preview, true, repo, head, "project.json")
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome.Kind).To(Equal(codesignalcli.SetupOutcomeFailed))
			Expect(outcome.PostInstallReadiness).To(BeNil(), "a failed install must never trigger a post-install readiness recheck")
		})
	})

	When("the install succeeds but the recheck itself cannot run", func() {
		It("returns the install's own success Kind alongside the recheck error, with PostInstallReadiness left nil", func() {
			GinkgoT().Setenv("PATH", pathWithStubNode("v24.9.9"))

			repo := newTempGitRepo()
			version := setupRecheckSupportedTypescriptVersion()
			commitFile(repo, "package.json", fmt.Sprintf(`{"name":"example","version":"1.0.0","devDependencies":{"typescript":%q}}`+"\n", version))
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			stubDir := writeInstallingSetupExecutable("npm", version)
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+pathWithStubNode("v24.9.9"))

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", repo)
			Expect(err).NotTo(HaveOccurred())

			// A revision that does not exist in the repo: the install itself
			// (which operates on the real working directory, not a revision)
			// still succeeds, but CheckProjectReadiness's git-snapshot read
			// cannot resolve it, so the recheck errors.
			nonexistentRevision := "0000000000000000000000000000000000000000"

			outcome, err := codesignalcli.RunConfirmedSetupAndRecheckReadiness(context.Background(), preview, true, repo, nonexistentRevision, "project.json")
			Expect(err).To(HaveOccurred(), "the recheck must surface its own failure to resolve the given revision")
			Expect(outcome.Kind).To(Equal(codesignalcli.SetupOutcomeSucceeded), "the install itself succeeded; only the recheck failed")
			Expect(outcome.PostInstallReadiness).To(BeNil(), "a failed recheck must never attach a zero-valued or partial readiness result")
		})
	})
})
