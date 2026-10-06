package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript", func() {
	When("HEAD has a TypeScript-shaped project (package.json) but no project.json policy, and Node is a supported major", func() {
		It("exits 0, reports status needs_policy, policy fail/policy_missing, compiler pass, and an author_policy next action (JSON)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")
			requireStubNodeVersion(path, "v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.SchemaVersion).To(Equal("1"))
			Expect(doc.Language).To(Equal("typescript"))
			Expect(doc.Status).To(Equal("needs_policy"))
			Expect(doc.Checks.Policy.State).To(Equal("fail"))
			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(doc.Checks.PackageManager.State).To(Equal("not_checked"))
			Expect(doc.Checks.PackageManager.Code).To(BeEmpty())
			Expect(gapCodes(doc)).NotTo(ContainElement("package_manager_ambiguous"))
			Expect(gapCodes(doc)).NotTo(ContainElement("package_manager_config_unverifiable"))
			Expect(doc.Checks.Node.State).To(Equal("pass"), "the stubbed, supported Node major must report pass deterministically")
			Expect(gapCodes(doc)).To(ContainElement("policy_missing"))
			Expect(nextActionKinds(doc)).To(ContainElement("author_policy"))
		})

		It("renders the same status, gaps, and next actions in the default text format", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			text := string(stdout)
			Expect(text).To(ContainSubstring("status: needs_policy"))
			Expect(text).To(ContainSubstring("policy: fail (policy_missing)"))
			Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
			Expect(text).To(ContainSubstring("package_manager: not_checked"))
			Expect(text).To(ContainSubstring("author_policy"))
		})
	})

	When("HEAD has a committed, valid project.json policy plus tsconfig.json and package.json, and Node is a supported major", func() {
		It("reports the policy check as pass and, with every other check clean, an overall status of ready", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("pass"))
			Expect(doc.Checks.Policy.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(doc.Checks.PackageManager.State).To(Equal("not_checked"))
			Expect(doc.Checks.PackageManager.Code).To(BeEmpty())
			Expect(gapCodes(doc)).To(BeEmpty())
			Expect(doc.Status).To(Equal("ready"))
		})

		It("reports the same policy pass when --project-config is omitted, reading the default project.json at HEAD", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			explicitStdout, explicitStderr, explicitExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(explicitExit).To(Equal(0), "stderr: %s", explicitStderr)
			omittedStdout, omittedStderr, omittedExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(omittedExit).To(Equal(0), "stderr: %s", omittedStderr)

			var explicitDoc, omittedDoc readinessResultDoc
			Expect(json.Unmarshal(explicitStdout, &explicitDoc)).To(Succeed(), "stdout: %s", explicitStdout)
			Expect(json.Unmarshal(omittedStdout, &omittedDoc)).To(Succeed(), "stdout: %s", omittedStdout)
			Expect(omittedDoc).To(Equal(explicitDoc))
			Expect(omittedDoc.Checks.Policy.State).To(Equal("pass"))
			Expect(omittedDoc.Status).To(Equal("ready"))
		})
	})

	When("coach is invoked from a committed subdirectory of the repository rather than the repository root", func() {
		It("resolves package.json and the --project-config policy against the repository root, reporting the same ready verdict as from the root", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "sub/marker.txt", "committed subdirectory\n")

			path := pathWithStubNode("v24.9.9")

			rootStdout, rootStderr, rootExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(rootExit).To(Equal(0), "stderr: %s", rootStderr)
			var rootDoc readinessResultDoc
			Expect(json.Unmarshal(rootStdout, &rootDoc)).To(Succeed(), "stdout: %s", rootStdout)

			subDir := filepath.Join(repo, "sub")
			subStdout, subStderr, subExit := runCoachCheckProjectEnv(subDir, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(subExit).To(Equal(0), "stderr: %s", subStderr)
			var subDoc readinessResultDoc
			Expect(json.Unmarshal(subStdout, &subDoc)).To(Succeed(), "stdout: %s", subStdout)

			Expect(subDoc.Checks.ProjectShape).To(Equal(rootDoc.Checks.ProjectShape))
			Expect(subDoc.Checks.Policy).To(Equal(rootDoc.Checks.Policy))
			Expect(subDoc.Checks.ProjectShape.State).To(Equal("pass"))
			Expect(subDoc.Checks.Policy.State).To(Equal("pass"))
			Expect(gapCodes(subDoc)).To(BeEmpty())
			Expect(subDoc.Status).To(Equal("ready"))
		})
	})

	When("a relevant file is modified in the worktree without being committed, while HEAD already has a real (policy) gap", func() {
		It("reports dirty_worktree.relevant_changes true with the path, as a warning that never overrides the HEAD-derived needs_policy status", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			cleanStdout, cleanStderr, cleanExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(cleanExit).To(Equal(0), "stderr: %s", cleanStderr)
			var cleanDoc readinessResultDoc
			Expect(json.Unmarshal(cleanStdout, &cleanDoc)).To(Succeed())
			Expect(cleanDoc.Status).To(Equal("needs_policy"))
			Expect(cleanDoc.DirtyWorktree.RelevantChanges).To(BeFalse())

			Expect(os.WriteFile(filepath.Join(repo, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true}}`+"\n"), 0o644)).To(Succeed())

			dirtyStdout, dirtyStderr, dirtyExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(dirtyExit).To(Equal(0), "stderr: %s", dirtyStderr)
			var dirtyDoc readinessResultDoc
			Expect(json.Unmarshal(dirtyStdout, &dirtyDoc)).To(Succeed())

			Expect(dirtyDoc.DirtyWorktree.RelevantChanges).To(BeTrue())
			Expect(dirtyDoc.DirtyWorktree.Paths).To(ContainElement("tsconfig.json"))
			Expect(gapCodes(dirtyDoc)).To(Equal(gapCodes(cleanDoc)), "a dirty worktree must never itself become a gap")
			Expect(dirtyDoc.Status).To(Equal("needs_policy"), "the uncommitted tsconfig.json edit must never override the HEAD-derived needs_policy status, proving the worktree change never becomes analysis input")
			Expect(dirtyDoc.Checks.Policy).To(Equal(cleanDoc.Checks.Policy), "worktree content must never leak into a snapshot check's result")
		})
	})

	When("a relevant file is modified in the worktree without being committed, while HEAD is otherwise fully ready (no gaps)", func() {
		It("elevates status to ready_with_limits, since that limit class is itself part of the frozen precedence", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			cleanStdout, cleanStderr, cleanExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(cleanExit).To(Equal(0), "stderr: %s", cleanStderr)
			var cleanDoc readinessResultDoc
			Expect(json.Unmarshal(cleanStdout, &cleanDoc)).To(Succeed())
			Expect(gapCodes(cleanDoc)).To(BeEmpty(), "sanity check: this fixture must have no real gaps so the limit class alone decides status")
			Expect(cleanDoc.Status).To(Equal("ready"))

			Expect(os.WriteFile(filepath.Join(repo, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true}}`+"\n"), 0o644)).To(Succeed())

			dirtyStdout, dirtyStderr, dirtyExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(dirtyExit).To(Equal(0), "stderr: %s", dirtyStderr)
			var dirtyDoc readinessResultDoc
			Expect(json.Unmarshal(dirtyStdout, &dirtyDoc)).To(Succeed())

			Expect(dirtyDoc.Status).To(Equal("ready_with_limits"))
			Expect(gapCodes(dirtyDoc)).To(BeEmpty(), "the dirty worktree must never itself become a gap")
		})

		It("renders the exact warning wording in the default text format, listing the uncommitted path", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			Expect(os.WriteFile(filepath.Join(repo, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true}}`+"\n"), 0o644)).To(Succeed())

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			text := string(stdout)
			Expect(text).To(ContainSubstring("Warning: uncommitted or untracked changes exist under paths relevant to this result. "))
			Expect(text).To(ContainSubstring("These paths are not part of the analyzed revision and had no effect on the checks above:"))
			Expect(text).To(ContainSubstring("tsconfig.json"))
		})
	})

	When("node_modules is gitignored and present only as untracked worktree files", func() {
		It("does not list those paths as relevant dirty worktree changes", func() {
			repo := newTempGitRepo()
			commitFile(repo, ".gitignore", "node_modules\n")
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			Expect(os.MkdirAll(filepath.Join(repo, "node_modules", "pkg"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "node_modules", "pkg", "index.js"), []byte("module.exports = {}\n"), 0o644)).To(Succeed())

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Status).To(Equal("ready"))
			Expect(doc.DirtyWorktree.RelevantChanges).To(BeFalse())
			Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(HavePrefix("node_modules")))
		})
	})

	DescribeTable("lists an uncommitted package-manager setup input at the repository root as a relevant dirty path even when the selected root is nested (SA-280-027)",
		func(relPath, contents string) {
			repo := newTempGitRepo()
			commitFile(repo, "app/package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescriptUnder(repo, "app", "7.0.2")
			commitFile(repo, "app/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["app"]}`+"\n")
			writeWorktreeFile(repo, relPath, contents)
			writeWorktreeFile(repo, "unrelated.txt", "not a setup input\n")

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.DirtyWorktree.RelevantChanges).To(BeTrue())
			Expect(doc.DirtyWorktree.Paths).To(ContainElement(relPath),
				"readiness reads this file as a package-manager setup input, so an uncommitted copy must be listed even when it sits outside the selected root")
			Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement("unrelated.txt"),
				"a nested selected root must not make every uncommitted path relevant")
		},
		Entry(".npmrc", ".npmrc", "# comment-only npmrc; not a registry redirect\n"),
		Entry("bunfig.toml", "bunfig.toml", "telemetry = false\n"),
		Entry("bun.lock", "bun.lock", "{\n  \"lockfileVersion\": 0,\n}\n"),
	)

	When("HEAD has a committed directory named package.json and no package.json blob, with no policy committed", func() {
		It("reports project_shape not_checked rather than unsupported_repository_shape, since without a validated policy this check has no basis to condemn the shape (R1)", func() {
			repo := newTempGitRepo()
			Expect(os.Mkdir(filepath.Join(repo, "package.json"), 0o755)).To(Succeed())
			commitFile(repo, "package.json/inner.txt", "not a manifest\n")

			path := pathWithStubNode("v24.9.9")
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.ProjectShape.State).To(Equal("not_checked"))
			Expect(doc.Checks.ProjectShape.Code).To(BeEmpty())
			Expect(doc.Status).NotTo(Equal("outside_support"))
		})
	})

	When("HEAD has no committed package.json at all, and no policy is committed either", func() {
		It("exits 0 and reports project_shape not_checked, never unsupported_repository_shape or confirm_repository_shape, since this check cannot tell a genuinely unsupported shape apart from an as-yet-uncommitted monorepo policy (R1)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "README.md", "no package.json here\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Status).NotTo(Equal("outside_support"))
			Expect(doc.Checks.ProjectShape.State).To(Equal("not_checked"))
			Expect(doc.Checks.ProjectShape.Code).To(BeEmpty())
			Expect(gapCodes(doc)).NotTo(ContainElement("unsupported_repository_shape"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("confirm_repository_shape"))
			Expect(gapCodes(doc)).To(ContainElement("policy_missing"))
		})
	})

	When("HEAD is a two-package monorepo (each package under packages/<name>) with no root package.json, no committed policy, and no toolchain at all reachable on PATH", func() {
		It("reports status needs_prerequisite with policy_missing and typescript_compiler_missing, never unsupported_repository_shape or a package_manager_* gap sourced from checkPackageManager's own worktree-root fallback (R1)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "packages/app/package.json", `{"name":"app","version":"1.0.0"}`+"\n")
			commitFile(repo, "packages/app/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "packages/app/src/index.ts", "export const x = 1;\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Status).To(Equal("needs_prerequisite"))
			Expect(doc.Checks.ProjectShape.State).To(Equal("not_checked"))
			Expect(doc.Checks.PackageManager.State).To(Equal("not_checked"))
			Expect(gapCodes(doc)).To(ContainElement("policy_missing"))
			Expect(gapCodes(doc)).To(ContainElement("typescript_compiler_missing"))
			Expect(gapCodes(doc)).NotTo(ContainElement("unsupported_repository_shape"))

			for _, gap := range doc.Gaps {
				if strings.HasPrefix(gap.Code, "package_manager_") {
					Expect(gap.PackageManagerKind).To(HavePrefix("mise_"), "a bare package_manager_* gap with no mise_* kind would mean checkPackageManager's own worktree-root fallback fired without a validated policy")
				}
			}
		})
	})

	When("HEAD has a committed, valid project.json policy declaring a non-root root, with package.json only under that root", func() {
		It("reports project_shape pass and a status other than outside_support, since the declared root names exactly where the TypeScript project lives", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["sub"]}`+"\n")
			commitFile(repo, "sub/package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "sub/tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("pass"))
			Expect(doc.Checks.ProjectShape.State).To(Equal("pass"))
			Expect(doc.Checks.ProjectShape.Code).To(BeEmpty())
			Expect(doc.Status).NotTo(Equal("outside_support"))
			Expect(gapCodes(doc)).NotTo(ContainElement("unsupported_repository_shape"))
		})
	})

	When("HEAD has a committed, valid project.json policy declaring a non-root root, with no package.json under that root or the repository root", func() {
		It("still reports project_shape fail/unsupported_repository_shape and status outside_support, proving a passing policy alone is never trusted without a real package.json hit", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["sub"]}`+"\n")
			commitFile(repo, "sub/tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("pass"))
			Expect(doc.Checks.ProjectShape.State).To(Equal("fail"))
			Expect(doc.Checks.ProjectShape.Code).To(Equal("unsupported_repository_shape"))
			Expect(doc.Status).To(Equal("outside_support"))
			Expect(gapCodes(doc)).To(ContainElement("unsupported_repository_shape"))
		})
	})

	When("HEAD has an invalid project.json policy (policy_invalid) declaring a non-root root, with package.json present under that root", func() {
		It("reports project_shape not_checked end-to-end, when checkPolicy's invalid-policy path also returns nil roots, rather than condemning a shape this check never got to walk (R1)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"2","roots":["sub"]}`+"\n")
			commitFile(repo, "sub/package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "sub/tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("fail"))
			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(doc.Checks.ProjectShape.State).To(Equal("not_checked"))
			Expect(doc.Checks.ProjectShape.Code).To(BeEmpty())
			Expect(doc.Status).NotTo(Equal("outside_support"))
			Expect(gapCodes(doc)).NotTo(ContainElement("unsupported_repository_shape"))
		})
	})

	When("HEAD has a committed project.json policy whose roots list exceeds the roots-count budget", func() {
		It("reports policy fail/policy_invalid and exits 0 quickly, rather than fanning out into a git child process per declared root", func() {
			repo := newTempGitRepo()

			const oversizedRootsCount = 257
			var roots strings.Builder
			roots.WriteString(`{"schema_version":"1","roots":[`)
			for i := 0; i < oversizedRootsCount; i++ {
				if i > 0 {
					roots.WriteByte(',')
				}
				fmt.Fprintf(&roots, `"root%d"`, i)
			}
			roots.WriteString(`]}` + "\n")
			commitFile(repo, "project.json", roots.String())
			By("relying on a locatable project mise.toml compiler so the compiler check passes cleanly, isolating this assertion to the roots budget")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, _ := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

			started := time.Now()
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			elapsed := time.Since(started)
			Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)
			Expect(elapsed).To(BeNumerically("<", 10*time.Second), "an oversized roots list must be rejected before fanning out into a git child process per root; elapsed=%s", elapsed)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("fail"))
			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(doc.Checks.ProjectShape.State).To(Equal("not_checked"), "with the policy rejected, project_shape has no validated roots to walk and reports not_checked instead of guessing (R1)")
			Expect(doc.Checks.ProjectShape.Code).To(BeEmpty())
			Expect(doc.Status).To(Equal("needs_policy"))
			Expect(gapCodes(doc)).To(ConsistOf("policy_invalid"))
		})
	})

	When("two independently discoverable gaps exist at once (policy_missing and node_unsupported)", func() {
		It("reports both gaps in gaps[], with status reflecting only the higher-precedence one", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")

			path := pathWithStubNode("v22.10.0")
			requireStubNodeVersion(path, "v22.10.0")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Status).To(Equal("needs_prerequisite"), "node_unsupported must outrank the simultaneous policy_missing gap")
			Expect(gapCodes(doc)).To(Equal([]string{"policy_missing", "node_unsupported"}), "both independently discoverable gaps must be reported, never hidden by precedence")
			Expect(nextActionKinds(doc)).To(ContainElements("author_policy", "install_supported_runtime"))
		})
	})

	When("the repository cannot be read at the resolved revision (a corrupt/missing committed blob)", func() {
		It("exits 1 and never emits a readiness document, rather than reporting a confident but false verdict", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			corruptCommittedBlob(repo, "package.json")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(1), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "an operational failure must never emit a readiness JSON document")
			Expect(string(stderr)).NotTo(BeEmpty())
		})
	})

	When("the repository cannot resolve a committed subtree at the resolved revision (a corrupt/missing tree object)", func() {
		It("exits 1 and never emits a readiness document, rather than reporting the committed policy file as missing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "config/project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			corruptCommittedTree(repo, "config")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "config/project.json", "--format", "json")
			Expect(exitCode).To(Equal(1), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "an operational failure must never emit a readiness JSON document")
			Expect(string(stderr)).NotTo(BeEmpty())
		})
	})

	When("HEAD has a committed project.json policy larger than the git-read size budget", func() {
		It("reports policy fail/policy_invalid and exits 0, treating the oversized (customer-fixable) file as a gap rather than an operational failure", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", strings.Repeat("a", (1<<20)+16))

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("fail"))
			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(doc.Status).To(Equal("needs_policy"))
		})
	})

	DescribeTable("invalid --check-project argument combinations exit 2 for the specific reason validateCheckProjectFlags reports",
		func(args []string, wantMessage string) {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")

			stdout, stderr, exitCode := runCoachSuggest(repo, args...)
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring(wantMessage))
		},
		Entry("missing --baseline and --project-language",
			[]string{"--check-project"},
			"coach: --check-project requires --baseline"),
		Entry("missing --baseline",
			[]string{"--check-project", "--project-language", "typescript"},
			"coach: --check-project requires --baseline"),
		Entry("missing --project-language",
			[]string{"--baseline", "--check-project"},
			`coach: --check-project requires --project-language typescript (got "go")`),
		Entry("--project-language go is not typescript",
			[]string{"--baseline", "--check-project", "--project-language", "go"},
			`coach: --check-project requires --project-language typescript (got "go")`),
		Entry("duplicate --check-project",
			[]string{"--baseline", "--check-project", "--project-language", "typescript", "--check-project"},
			"coach: --check-project may only be provided once"),
		Entry("--check-project cannot be combined with --scope",
			[]string{"--baseline", "--check-project", "--project-language", "typescript", "--scope", "all"},
			"coach: --check-project cannot be combined with --scope"),
		Entry("--project-config is an absolute path",
			[]string{"--baseline", "--check-project", "--project-language", "typescript", "--project-config", "/etc/passwd"},
			`coach: --project-config "/etc/passwd" is invalid: path must be a non-empty repository-relative path`),
		Entry("--project-config escapes the repository via ..",
			[]string{"--baseline", "--check-project", "--project-language", "typescript", "--project-config", "../../etc/passwd"},
			`coach: --project-config "../../etc/passwd" is invalid: path must be normalized and remain inside the repository`),
	)
})
