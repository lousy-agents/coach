package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

type readinessRootFindingDoc struct {
	Root    string `json:"root"`
	Version string `json:"version"`
}

type readinessCheckDoc struct {
	State             string                    `json:"state"`
	Code              string                    `json:"code"`
	Kind              string                    `json:"kind"`
	Version           string                    `json:"version"`
	ExpectedVersion   string                    `json:"expected_version"`
	FoundVersion      string                    `json:"found_version"`
	PinnedVersion     string                    `json:"pinned_version"`
	SupportedVersions []string                  `json:"supported_versions"`
	RootFindings      []readinessRootFindingDoc `json:"root_findings"`
	Origin            string                    `json:"origin"`
	Detail            string                    `json:"detail"`
}

type readinessResultDoc struct {
	SchemaVersion string `json:"schema_version"`
	Status        string `json:"status"`
	Language      string `json:"language"`
	Revision      string `json:"revision"`
	DirtyWorktree struct {
		RelevantChanges bool     `json:"relevant_changes"`
		Paths           []string `json:"paths"`
	} `json:"dirty_worktree"`
	Checks struct {
		ProjectShape   readinessCheckDoc `json:"project_shape"`
		Policy         readinessCheckDoc `json:"policy"`
		Node           readinessCheckDoc `json:"node"`
		Runtime        readinessCheckDoc `json:"runtime"`
		Compiler       readinessCheckDoc `json:"compiler"`
		PackageManager readinessCheckDoc `json:"package_manager"`
	} `json:"checks"`
	Gaps []struct {
		Code               string `json:"code"`
		PackageManagerKind string `json:"package_manager_kind"`
	} `json:"gaps"`
	Warnings []struct {
		Code              string `json:"code"`
		DeclaredVersion   string `json:"declared_version"`
		FoundVersion      string `json:"found_version"`
		DeclarationOrigin string `json:"declaration_origin"`
		Root              string `json:"root"`
	} `json:"warnings"`
	NextActions []readinessNextActionDoc `json:"next_actions"`
}

type readinessNextActionDoc struct {
	Kind               string   `json:"kind"`
	Executable         bool     `json:"executable"`
	RuntimeKind        string   `json:"runtime_kind"`
	PackageManagerKind string   `json:"package_manager_kind"`
	Supported          []string `json:"supported"`
	FoundVersion       string   `json:"found_version"`
	Detail             string   `json:"detail"`
	Choices            []string `json:"choices"`
}

const (
	stubPackageManagerCwdLog = "cwd.log"
	stubPackageManagerEnvLog = "env.log"
)

const stubMiseInvocationLog = "mise-invocations.log"

const stubMiseCwdLog = "mise-probe-cwd.log"

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
			body_projectReadinessAcceptanceTest_reportsStatusNeedsPrerequisiteWithPolicyMissingA_391()
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
			body_projectReadinessAcceptanceTest_reportsPolicyFailPolicyInvalidAndExits0QuicklyRa_486()
		})
	})

	When("Node cannot be found on the child process's PATH at all", func() {
		It("reports the node check as fail/node_missing deterministically, mirrored exactly by checks.runtime, with an install_supported_runtime next action", func() {
			body_projectReadinessAcceptanceTest_reportsTheNodeCheckAsFailNodeMissingDeterministi_523()
		})
	})

	When("the resolvable Node's major version is outside the supported set, below every member", func() {
		It("reports the node check as fail/node_unsupported with the found version, deterministically", func() {
			body_projectReadinessAcceptanceTest_reportsTheNodeCheckAsFailNodeUnsupportedWithTheF_572()
		})
	})

	When("the resolvable Node's major version is outside the supported set, above every member", func() {
		It("reports the node check as fail/node_unsupported, proving the set membership swap fires above the set too, not merely below a retired floor", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v27.0.0")
			requireStubNodeVersion(path, "v27.0.0")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_unsupported"))
			Expect(doc.Checks.Node.Version).To(Equal("v27.0.0"))
			Expect(doc.Checks.Runtime.Code).To(Equal("node_unsupported"))
			Expect(doc.Status).To(Equal("needs_prerequisite"))
		})
	})

	When("Node resolves to the supported major 24", func() {
		It("reports the node check as pass with no code, warning, or gap, and checks.runtime carries kind/origin", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")
			requireStubNodeVersion(path, "v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("pass"))
			Expect(doc.Checks.Node.Code).To(BeEmpty())
			Expect(doc.Checks.Node.Version).To(Equal("v24.9.9"))
			Expect(doc.Checks.Node.Kind).To(BeEmpty())
			Expect(doc.Checks.Node.Origin).To(BeEmpty())
			Expect(doc.Checks.Runtime.State).To(Equal("pass"))
			Expect(doc.Checks.Runtime.Code).To(BeEmpty())
			Expect(doc.Checks.Runtime.Version).To(Equal("v24.9.9"))
			Expect(doc.Checks.Runtime.Kind).To(Equal("node"))
			Expect(doc.Checks.Runtime.Origin).To(Equal("path"))
			Expect(doc.Status).To(Equal("ready"))
			Expect(doc.Warnings).To(BeEmpty(), "a supported Node major must never emit a warning")
		})
	})

	When("Node resolves to the supported major 26", func() {
		It("reports the node check as pass with no code, warning, or gap, proving 26 is a first-class supported major rather than an untested one", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v26.0.0")
			requireStubNodeVersion(path, "v26.0.0")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("pass"))
			Expect(doc.Checks.Node.Code).To(BeEmpty())
			Expect(doc.Checks.Node.Version).To(Equal("v26.0.0"))
			Expect(doc.Checks.Runtime.State).To(Equal("pass"))
			Expect(doc.Checks.Runtime.Code).To(BeEmpty())
			Expect(doc.Checks.Runtime.Version).To(Equal("v26.0.0"))
			Expect(doc.Status).To(Equal("ready"))
			Expect(doc.Warnings).To(BeEmpty(), "a supported Node major must never emit a warning, and node_untested no longer exists")

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)
			Expect(text).To(ContainSubstring("status: ready"))
			Expect(text).To(ContainSubstring("runtime: pass kind=node version=v26.0.0 origin=path"))
			Expect(text).NotTo(ContainSubstring("node_untested"))
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

	When("Node is on PATH but `node --version` hangs indefinitely", func() {
		It("still exits within a bounded wall clock, reporting node fail/node_unverifiable with a bounded stderr-free detail, and still produces the fit report", func() {
			body_projectReadinessAcceptanceTest_stillExitsWithinABoundedWallClockReportingNodeFa_771()
		})
	})

	When("Node is on PATH but `node --version` exits non-zero", func() {
		It("reports node fail/node_unverifiable with a bounded path-free detail and a repair_runtime_probe next action, not node_missing", func() {
			body_projectReadinessAcceptanceTest_reportsNodeFailNodeUnverifiableWithABoundedPathF_818()
		})
	})

	When("Node is on PATH but the `node --version` process cannot start", func() {
		It("reports node fail/node_unverifiable with a bounded, path-free start-failure detail and a repair_runtime_probe next action", func() {
			body_projectReadinessAcceptanceTest_reportsNodeFailNodeUnverifiableWithABoundedPathF_865()
		})
	})

	When("Node resolves and runs but prints output that is not a parsable version string", func() {
		It("reports node fail/node_unverifiable naming the raw observed output, not node_missing or node_unsupported", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("weird-build-2024")
			requireStubNodeVersion(path, "weird-build-2024")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.SchemaVersion).NotTo(BeEmpty(), "the readiness document must still be produced on a probe failure")
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_unverifiable"))
			Expect(doc.Checks.Runtime.State).To(Equal("fail"))
			Expect(doc.Checks.Runtime.Code).To(Equal("node_unverifiable"))
			Expect(doc.Checks.Runtime.Detail).To(ContainSubstring("weird-build-2024"))
			Expect(doc.Checks.Runtime.Detail).NotTo(ContainSubstring(string(os.PathSeparator)), "the detail must not leak the stub node's host path")
			Expect(doc.Checks.Runtime.Version).To(BeEmpty(), "an unverifiable probe must not report a confirmed version")
			Expect(doc.Status).To(Equal("needs_prerequisite"))
		})
	})

	When("Node resolves and runs but prints an oversized unparsable version blob", func() {
		It("reports node fail/node_unverifiable with a detail bounded well below the raw probe-output budget", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithOversizedUnparsableNode()

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Runtime.State).To(Equal("fail"))
			Expect(doc.Checks.Runtime.Code).To(Equal("node_unverifiable"))
			Expect(doc.Checks.Runtime.Detail).NotTo(BeEmpty())
			Expect(doc.Checks.Runtime.Detail).To(HavePrefix("node --version printed an unparsable version:"), "must land on the rawVersion-embedding branch, not the exceeded-probe-budget default")
			Expect(doc.Checks.Runtime.Detail).To(ContainSubstring("...(truncated)"), "the oversized rawVersion must actually have been truncated")
			Expect(len(doc.Checks.Runtime.Detail)).To(BeNumerically("<", 1<<10), "the probe-failure detail must be bounded even when the probed output is not")
			Expect(doc.Status).To(Equal("needs_prerequisite"))
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

var _ = Describe("coach codesignal --baseline --prepare-compiler --project-language typescript", func() {
	DescribeTable("invalid --prepare-compiler argument combinations exit 2 for the specific reason validatePrepareCompilerFlags reports",
		func(args []string, wantMessage string) {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, args...)
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring(wantMessage))
		},
		Entry("missing --baseline and --project-language",
			[]string{"--prepare-compiler"},
			"coach: --prepare-compiler requires --baseline"),
		Entry("missing --baseline",
			[]string{"--prepare-compiler", "--project-language", "typescript"},
			"coach: --prepare-compiler requires --baseline"),
		Entry("missing --project-language",
			[]string{"--baseline", "--prepare-compiler"},
			`coach: --prepare-compiler requires --project-language typescript (got "go")`),
		Entry("--project-language go is not typescript",
			[]string{"--baseline", "--prepare-compiler", "--project-language", "go"},
			`coach: --prepare-compiler requires --project-language typescript (got "go")`),
		Entry("duplicate --prepare-compiler",
			[]string{"--baseline", "--prepare-compiler", "--project-language", "typescript", "--prepare-compiler"},
			"coach: --prepare-compiler may only be provided once"),
		Entry("--prepare-compiler cannot be combined with --check-project",
			[]string{"--baseline", "--prepare-compiler", "--project-language", "typescript", "--check-project"},
			"coach: --prepare-compiler cannot be combined with --check-project"),
		Entry("--prepare-compiler does not accept positional arguments",
			[]string{"--baseline", "--prepare-compiler", "--project-language", "typescript", "extra-positional-arg"},
			"coach: --prepare-compiler does not accept positional arguments"),
		Entry("--project-config is an absolute path",
			[]string{"--baseline", "--prepare-compiler", "--project-language", "typescript", "--project-config", "/etc/passwd"},
			`coach: --project-config "/etc/passwd" is invalid: path must be a non-empty repository-relative path`),
	)

	When("--prepare-compiler is supplied with --baseline and --project-language typescript, but stdin has no controlling terminal", func() {
		It("reaches the real interactive compiler-setup dispatch, which refuses without a controlling terminal, proving the flag is genuinely wired into the compiled binary's flag parser (AC-SET-24)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--prepare-compiler", "--project-language", "typescript")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("no controlling terminal is available"))
			Expect(string(stderr)).To(ContainSubstring("coach codesignal --baseline --prepare-compiler --project-language typescript"))
		})
	})
})

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: compiler resolution", func() {
	When("the committed package.json declares a unique exact typescript version", func() {
		It("reports the compiler check as pass with the resolved version, from the project manifest origin", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
		})
	})

	When("package.json declares no typescript version, but the worktree's project mise.toml pins a unique exact npm:typescript version", func() {
		It("reports the compiler check as pass with that version, from the project mise origin", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, _ := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
		})
	})

	When("neither package.json nor a project mise.toml name a typescript version, but the host's global mise configuration pins a unique exact npm:typescript version", func() {
		It("reports the compiler check as pass with that version, from the global mise origin, using only a read-only mise invocation", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")

			path, miseDir := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(readStubMiseInvocations(miseDir)).To(Equal([]string{
				"--version",
				"--version",
				"config ls -J",
				"config get tools.npm:typescript -g",
				"where npm:typescript@7.0.2",
				"--version",
				"--version",
				"config ls -J",
				"config get tools.npm:typescript -g",
			}), "the frozen global-mise mechanic must invoke only read-only detection, trust, location, and pin-verification commands, never mise install/use or any other mutating subcommand; the duplication is evaluateCompilerOrigins' and evaluateMiseSetupChoices' independent trust probes plus the setup-choice pin check")
		})
	})

	When("package.json declares a unique exact typescript version and the worktree's project mise.toml also pins a different exact version", func() {
		It("resolves from the project manifest origin, pinning that project outranks project mise rather than merely being the only candidate present", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"9.9.9\"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"), "the project manifest origin must outrank a conflicting project mise.toml candidate")
		})
	})

	When("the worktree's project mise.toml pins a unique exact version and the host's global mise configuration pins a different exact version", func() {
		It("resolves from the project mise origin, pinning that project mise outranks global mise rather than merely being the only candidate present", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, _ := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"))
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"), "the project mise origin must outrank a conflicting global mise candidate")
		})
	})

	When("mise's default npm backend hoists the compiler to a symlink but not its native sibling (real mise 2026.9 layout, coach#392 REV-392-02)", func() {
		It("still reports the compiler check passing, resolving the native package beside the real, symlink-resolved directory", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			toolRoot := GinkgoT().TempDir()
			realDir := filepath.Join(toolRoot, ".mise", "typescript@7.0.2", "node_modules", "typescript")
			Expect(os.MkdirAll(realDir, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(realDir, "package.json"), []byte(`{"name":"typescript","version":"7.0.2"}`+"\n"), 0o644)).To(Succeed())
			nativeUnscoped := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
			realNativeDir := filepath.Join(filepath.Dir(realDir), "@typescript", nativeUnscoped)
			Expect(os.MkdirAll(realNativeDir, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(realNativeDir, "package.json"), []byte(fmt.Sprintf(`{"name":%q,"version":"7.0.2"}`+"\n", "@typescript/"+nativeUnscoped)), 0o644)).To(Succeed())

			symlinkParent := filepath.Join(toolRoot, "node_modules")
			Expect(os.MkdirAll(symlinkParent, 0o755)).To(Succeed())
			symlinkPath := filepath.Join(symlinkParent, "typescript")
			Expect(os.Symlink(realDir, symlinkPath)).To(Succeed())

			miseDir := GinkgoT().TempDir()

			script := fmt.Sprintf("#!/bin/sh\n"+
				"if [ \"$1\" = \"--version\" ]; then echo \"2026.9.5 linux-x64 (2026-09-10)\"; exit 0; fi\n"+
				"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo \"[]\"; exit 0; fi\n"+
				"if [ \"$1\" = \"where\" ]; then echo %q; exit 0; fi\n", toolRoot)
			Expect(os.WriteFile(filepath.Join(miseDir, "mise"), []byte(script), 0o755)).To(Succeed())

			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
		})
	})

	When("package.json's declared exact typescript version disagrees with the version actually installed under node_modules/typescript", func() {
		It("reports the compiler check as fail/typescript_version_mismatch with both versions recorded", func() {
			body_projectReadinessAcceptanceTest_reportsTheCompilerCheckAsFailTypescriptVersionMi_1197()
		})
	})

	When("package.json declares two different exact typescript versions across dependencies and devDependencies", func() {
		It("reports the compiler check as fail/typescript_version_conflict rather than silently picking one", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","dependencies":{"typescript":"7.0.2"},"devDependencies":{"typescript":"5.4.0"}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_conflict"))
			Expect(gapCodes(doc)).To(ContainElement("typescript_version_conflict"))
		})
	})

	When("a js/semantics-shaped repository has no top-level package.json and the policy names the nested project root that pins an exact installed compiler", func() {
		It("reports checks.compiler.state=pass from that nested manifest rather than typescript_compiler_missing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["js/semantics"]}`+"\n")
			commitFile(repo, "js/semantics/package.json", `{"name":"semantics","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "js/semantics/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			writeInstalledTypescriptUnder(repo, "js/semantics", "7.0.2")

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "nested-only fixture must not have a top-level package.json; that would exercise the already-green worktree-top origin")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("pass"), "the nested policy root must itself be valid so a compiler miss cannot be blamed on policy")
			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "the nested js/semantics package.json pin must certify the compiler, got state=%s code=%s version=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, doc.Checks.Compiler.Version, stdout)
			Expect(doc.Checks.Compiler.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
		})
	})

	When("the policy selects roots [\".\"] and an exact installed compiler exists only under a nested directory", func() {
		It("reports fail/typescript_compiler_missing rather than walking down to certify the nested pin", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "js/semantics/package.json", `{"name":"semantics","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescriptUnder(repo, "js/semantics", "7.0.2")

			_, statErr := os.Stat(filepath.Join(repo, "js/semantics/package.json"))
			Expect(statErr).NotTo(HaveOccurred(), "walk-down fixture must keep a nested pin that a walk-down resolver would find")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.ProjectShape.State).To(Equal("pass"), "a top-level package.json must keep this on the compiler check, not unsupported_repository_shape")
			Expect(doc.Checks.Compiler.State).To(Equal("fail"), "roots [\".\"] must not walk down to js/semantics, got state=%s code=%s version=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, doc.Checks.Compiler.Version, stdout)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(doc.Checks.Compiler.Version).NotTo(Equal("7.0.2"), "the nested pin must not be selected as a passing compiler")
			Expect(gapCodes(doc)).To(ContainElement("typescript_compiler_missing"))
		})
	})

	When("a js/semantics-shaped repository has no top-level package.json and the policy names a subdirectory of the nested project whose package.json lives only in the parent", func() {
		It("reports checks.compiler.state=pass from the parent manifest rather than typescript_compiler_missing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["js/semantics/src"]}`+"\n")
			commitFile(repo, "js/semantics/package.json", `{"name":"semantics","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "js/semantics/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "js/semantics/src/index.ts", "export const x = 1;\n")
			writeInstalledTypescriptUnder(repo, "js/semantics", "7.0.2")

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "walk-up fixture must not have a top-level package.json; that would exercise the already-green worktree-top origin")
			_, statErr = os.Stat(filepath.Join(repo, "js/semantics/src/package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "walk-up fixture must not place package.json at the selected root; that would exercise the already-green at-root origin")
			_, statErr = os.Stat(filepath.Join(repo, "js/semantics/package.json"))
			Expect(statErr).NotTo(HaveOccurred(), "walk-up fixture must keep the pin strictly above the selected root")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).To(Equal("pass"), "the nested policy root must itself be valid so a compiler miss cannot be blamed on policy")
			Expect(doc.Checks.ProjectShape.State).To(Equal("pass"), "walk-up to the parent package.json must certify project_shape the same way it certifies the compiler, got state=%s code=%s stdout=%s", doc.Checks.ProjectShape.State, doc.Checks.ProjectShape.Code, stdout)
			Expect(doc.Status).NotTo(Equal("outside_support"), "a nested project whose package.json lives only above the selected root must not be outside_support, got status=%s stdout=%s", doc.Status, stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "the parent js/semantics package.json pin must certify the compiler via walk-up, got state=%s code=%s version=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, doc.Checks.Compiler.Version, stdout)
			Expect(doc.Checks.Compiler.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
		})
	})

	When("two policy roots include one nested exact installed compiler and one nested root with no package.json, and there is no top-level package.json", func() {
		It("reports fail/typescript_version_conflict rather than silently certifying the pinned root", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["apps/web","apps/api"]}`+"\n")
			commitFile(repo, "apps/web/package.json", `{"name":"web","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "apps/api/index.ts", "export const api = 1;\n")
			writeInstalledTypescriptUnder(repo, "apps/web", "7.0.2")

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "mixed pass+empty must not share a top-level package.json")
			_, statErr = os.Stat(filepath.Join(repo, "apps/api/package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "the empty root must have no package.json; a second pin would exercise the already-green two-pin conflict")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"), "a selected root with no manifest must not be skipped, got state=%s code=%s version=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, doc.Checks.Compiler.Version, stdout)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_conflict"))
			Expect(doc.Checks.Compiler.ExpectedVersion).To(BeEmpty(), "conflict omits expected_version; root_findings is the sole machine-readable surface")
			Expect(doc.Checks.Compiler.FoundVersion).To(BeEmpty(), "conflict omits found_version; root_findings is the sole machine-readable surface")
			Expect(doc.Checks.Compiler.Version).NotTo(Equal("7.0.2"), "the pinned root must not be selected as a passing compiler")
			Expect(doc.Checks.Compiler.RootFindings).To(Equal([]readinessRootFindingDoc{
				{Root: "apps/web", Version: "7.0.2"},
				{Root: "apps/api"},
			}), "each selected root's finding must be named, got %+v stdout=%s", doc.Checks.Compiler.RootFindings, stdout)
			Expect(gapCodes(doc)).To(ContainElement("typescript_version_conflict"))
		})
	})

	When("two policy roots pin disagreeing exact typescript versions in distinct nested manifests with no top-level package.json", func() {
		It("reports fail/typescript_version_conflict naming each root's version in root_findings", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["apps/web","apps/api"]}`+"\n")
			commitFile(repo, "apps/web/package.json", `{"name":"web","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "apps/api/package.json", `{"name":"api","version":"1.0.0","devDependencies":{"typescript":"5.4.0"}}`+"\n")
			writeInstalledTypescriptUnder(repo, "apps/web", "7.0.2")
			writeInstalledTypescriptUnder(repo, "apps/api", "5.4.0")

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "two-root disagreement must not share a top-level package.json")
			webManifest, err := os.ReadFile(filepath.Join(repo, "apps/web/package.json"))
			Expect(err).NotTo(HaveOccurred())
			apiManifest, err := os.ReadFile(filepath.Join(repo, "apps/api/package.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(webManifest).NotTo(Equal(apiManifest), "two-root disagreement must be two distinct manifests, not one package.json reached twice")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"), "disagreeing per-root pins must fail closed, got state=%s code=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, stdout)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_conflict"))
			Expect(doc.Checks.Compiler.ExpectedVersion).To(BeEmpty(), "conflict omits expected_version; root_findings is the sole machine-readable surface, got expected=%q found=%q", doc.Checks.Compiler.ExpectedVersion, doc.Checks.Compiler.FoundVersion)
			Expect(doc.Checks.Compiler.FoundVersion).To(BeEmpty(), "conflict omits found_version")
			Expect(doc.Checks.Compiler.RootFindings).To(Equal([]readinessRootFindingDoc{
				{Root: "apps/web", Version: "7.0.2"},
				{Root: "apps/api", Version: "5.4.0"},
			}), "each selected root's finding must be named, got %+v stdout=%s", doc.Checks.Compiler.RootFindings, stdout)
			Expect(gapCodes(doc)).To(ContainElement("typescript_version_conflict"))

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			Expect(string(textStdout)).To(ContainSubstring("apps/web"), "text remediation must name the web root, got %s", textStdout)
			Expect(string(textStdout)).To(ContainSubstring("apps/api"), "text remediation must name the api root, got %s", textStdout)
		})
	})

	When("two policy roots pin the same exact installed typescript version in distinct nested manifests with no top-level package.json", func() {
		It("reports checks.compiler.state=pass rather than typescript_version_conflict", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["apps/web","apps/api"]}`+"\n")
			commitFile(repo, "apps/web/package.json", `{"name":"web","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "apps/api/package.json", `{"name":"api","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescriptUnder(repo, "apps/web", "7.0.2")
			writeInstalledTypescriptUnder(repo, "apps/api", "7.0.2")

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "two-root agreement must not share a top-level package.json")
			webManifest, err := os.ReadFile(filepath.Join(repo, "apps/web/package.json"))
			Expect(err).NotTo(HaveOccurred())
			apiManifest, err := os.ReadFile(filepath.Join(repo, "apps/api/package.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(webManifest).NotTo(Equal(apiManifest), "two-root agreement must be two distinct manifests that happen to share a pin")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "agreeing per-root pins must pass, got state=%s code=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, stdout)
			Expect(doc.Checks.Compiler.Code).To(BeEmpty())
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
		})
	})

	When("no exact compiler candidate exists in the project manifest, project mise config, or global mise config", func() {
		It("reports the compiler check as fail/typescript_compiler_missing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(doc.Checks.Compiler.ExpectedVersion).To(Equal("7.0.2"))
			Expect(gapCodes(doc)).To(ContainElement("typescript_compiler_missing"))
		})
	})

	When("the installed compiler is an exact 5.x version from its own package.json", func() {
		It("reports fail/typescript_version_mismatch with supported_versions [7.0.2] rather than pass", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"5.4.0"}}`+"\n")
			writeInstalledTypescript(repo, "5.4.0")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_mismatch"))
			Expect(doc.Checks.Compiler.ExpectedVersion).To(Equal("7.0.2"))
			Expect(doc.Checks.Compiler.FoundVersion).To(Equal("5.4.0"))
			Expect(doc.Checks.Compiler.SupportedVersions).To(Equal([]string{"7.0.2"}))
			Expect(gapCodes(doc)).To(ContainElement("typescript_version_mismatch"))

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			Expect(string(textStdout)).To(ContainSubstring("7.0.2"), "gap text must name the supported set, got %s", textStdout)
		})
	})

	When("package.json declares an exact out-of-set typescript version, 7.0.2 is installed at the project origin, and project mise supplies 7.0.2", func() {
		It("passes from the project origin, which outranks the mise pin, and warns that the manifest declares a stale exact version", func() {
			body_projectReadinessAcceptanceTest_passesFromTheProjectOriginWhichOutranksTheMisePi_1484()
		})
	})

	When("project mise.toml pins two different exact typescript versions and there is no project manifest candidate", func() {
		It("reports fail/typescript_version_conflict with root_findings rather than omitting the machine-readable surface", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = [\"7.0.2\", \"5.4.0\"]\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_conflict"))
			Expect(doc.Checks.Compiler.ExpectedVersion).To(BeEmpty(), "conflict omits expected_version")
			Expect(doc.Checks.Compiler.FoundVersion).To(BeEmpty(), "conflict omits found_version")
			Expect(doc.Checks.Compiler.RootFindings).NotTo(BeEmpty(), "root_findings is the sole machine-readable conflict surface, got %+v stdout=%s", doc.Checks.Compiler.RootFindings, stdout)
		})
	})

	When("the worktree mise.toml carries an env exec template that would write a sentinel", func() {
		It("produces no side effect during --check-project because mise probes use a neutral working directory", func() {
			body_projectReadinessAcceptanceTest_producesNoSideEffectDuringCheckProjectBecauseMis_1543()
		})
	})
})

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: package-manager setup scoping (SA-280-045)", func() {
	When("a committed yarn.lock exists and the project's TypeScript compiler already resolves as installed and supported", func() {
		It("reports checks.package_manager fail/package_manager_version_unsupported for information only, contributing no gap and no resolve_package_manager action", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "yarn.lock", "# THIS IS AN AUTOGENERATED FILE. DO NOT EDIT THIS FILE DIRECTLY.\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "the compiler must resolve for this fixture to isolate the SA-280-045 suppression rule, stdout=%s", stdout)
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"), "the rejected Yarn adapter is still reported on checks.package_manager for information, stdout=%s", stdout)
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_version_unsupported"))
			Expect(doc.Checks.PackageManager.Kind).To(Equal("yarn"))
			Expect(gapCodes(doc)).NotTo(ContainElement("package_manager_version_unsupported"), "a package_manager_* code must not become a gap while the compiler already resolves (SA-280-045), stdout=%s", stdout)
			Expect(nextActionKinds(doc)).NotTo(ContainElement("resolve_package_manager"))
		})
	})

	When("a committed yarn.lock exists and no TypeScript compiler resolves, with no verifiable mise origin on PATH", func() {
		It("reports package_manager_version_unsupported as a gap with a resolve_package_manager action naming yarn, and withholds prepare_compiler since no other installation choice is verified", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "yarn.lock", "# THIS IS AN AUTOGENERATED FILE. DO NOT EDIT THIS FILE DIRECTLY.\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(doc.Checks.PackageManager.State).To(Equal("fail"))
			Expect(doc.Checks.PackageManager.Code).To(Equal("package_manager_version_unsupported"))
			Expect(gapCodes(doc)).To(ContainElements("typescript_compiler_missing", "package_manager_version_unsupported"), "got %+v stdout=%s", doc.Gaps, stdout)
			Expect(doc.NextActions).To(ContainElement(HaveField("PackageManagerKind", "yarn")), "the resolve_package_manager action must name which adapter was rejected, got %+v stdout=%s", doc.NextActions, stdout)
			Expect(nextActionKinds(doc)).To(ContainElement("resolve_package_manager"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("prepare_compiler"), "prepare_compiler must be withheld when the only installation choice (the Yarn adapter) is rejected and no mise origin is verified (SA-280-045), got %+v stdout=%s", doc.NextActions, stdout)
		})
	})

	When("a committed yarn.lock exists and no TypeScript compiler resolves, rendered in the default text format", func() {
		It("keeps the package_manager_kind discriminator on both the Gaps line and the resolve_package_manager next-action line", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "yarn.lock", "# THIS IS AN AUTOGENERATED FILE. DO NOT EDIT THIS FILE DIRECTLY.\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			text := string(stdout)
			Expect(text).To(ContainSubstring("package_manager_version_unsupported package_manager_kind=yarn"), "the Gaps list must carry the same package_manager_kind discriminator as the JSON gaps[] entry (AC-12), got:\n%s", text)
			Expect(text).To(ContainSubstring("resolve_package_manager (executable=false) package_manager_kind=yarn"), "got:\n%s", text)
			Expect(text).To(ContainSubstring("\n  typescript_compiler_missing\n"), "a gap with no package_manager_kind must still render as a bare code, got:\n%s", text)
		})
	})
})

var _ = Describe("coach's interim standalone prepare_compiler mise setup dispatch (coach#328 Task 5, AC-SET-1..AC-SET-8/19/22/23/24)", func() {
	When("mise is trusted but neither scope declares an exact supported TypeScript version", func() {
		It("offers no installation choice, exits 0, and never invokes mise install (AC-SET-1, AC-23)", func() {
			repo := noSupportedCompilerRepo()
			path, miseDir := pathWithStatefulStubNodeAndMise("v24.9.9", "7.0.2")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			GinkgoT().Setenv("CI", "")

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			transcript := readStderr()
			Expect(exitCode).To(Equal(0), "stderr: %s", transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(transcript).To(ContainSubstring("no executable mise compiler-setup choice is currently offered"), "transcript: %s", transcript)
			Expect(transcript).NotTo(ContainSubstring("  - mise_project"), "a scope with no in-set pin cannot make post-install readiness pass, transcript: %s", transcript)
			Expect(transcript).NotTo(ContainSubstring("  - mise_global"), "transcript: %s", transcript)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "a non-executable choice must never invoke `mise install`")
		})
	})

	When("both mise_project and mise_global are executable and verified, and the user selects one and then declines the install confirmation", func() {
		It("shows the full AC-SET-2 preview naming both offered choices, then exits 2 with empty stdout and no mise mutation on decline (AC-SET-2, AC-SET-5, AC-SET-8)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			path, miseDir := pathWithStatefulStubNodeAndMiseGlobalAware("v24.9.9", "7.0.2")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			stdin := authoringStdin("mise_project\ndecline\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			Expect(exitCode).To(Equal(2))
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")

			transcript := readStderr()
			Expect(transcript).To(ContainSubstring("mise_project"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("mise_global"), "both verified choices must be listed so selection has no default, transcript: %s", transcript)

			Expect(transcript).To(ContainSubstring("Executable: mise"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Arguments: install npm:typescript@7.0.2"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Working directory:"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Expected mise changes:"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Network use:"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Lifecycle-script policy:"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Timeout: 5m0s"), "transcript: %s", transcript)

			Expect(transcript).To(ContainSubstring("cancelled"), "transcript: %s", transcript)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "a declined confirmation must never invoke `mise install`")
		})
	})

	When("the choice-selection answer names neither offered mise scope", func() {
		It("cancels without ever showing the install preview, proving there is no default choice (AC-SET-5, AC-SET-8)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			path, miseDir := pathWithStatefulStubNodeAndMiseGlobalAware("v24.9.9", "7.0.2")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			stdin := authoringStdin("not-a-real-choice\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			Expect(exitCode).To(Equal(2))
			Expect(readStdout()).To(BeEmpty())

			transcript := readStderr()
			Expect(transcript).NotTo(ContainSubstring("Executable: mise"), "an unrecognized selection must never reach the install preview, transcript: %s", transcript)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "an unrecognized selection must never invoke `mise install`")
		})
	})

	When("there is no controlling terminal on stdin", func() {
		It("never prompts, never mutates mise state, and exits 2 (AC-SET-24)", func() {
			repo := noSupportedCompilerRepo()
			path, miseDir := pathWithStatefulStubNodeAndMise("v24.9.9", "7.0.2")

			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			GinkgoT().Setenv("CI", "")

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := runPrepareCompilerMiseTypeScript(repo, codesignalFlags{}, stdin, stdoutFile, stderrFile)

			Expect(exitCode).To(Equal(2))
			Expect(readStdout()).To(BeEmpty())
			Expect(readStderr()).To(ContainSubstring("controlling terminal"))
			_, statErr := os.Stat(filepath.Join(miseDir, stubMiseInvocationLog))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "no controlling terminal must mean mise is never invoked at all, not even for a read-only probe")
		})
	})

	When("both a policy gap and a compiler gap exist (AC-SET-13)", func() {
		It("lists no mise installation choice, exits 2, and stderr names author_policy as the required first action", func() {
			body_projectReadinessAcceptanceTest_listsNoMiseInstallationChoiceExits2AndStderrName_1748()
		})
	})

	When("the project mise.toml already declares the frozen TypeScript version but it is not yet installed, and the user selects mise_project and confirms", func() {
		It("installs it, reruns readiness, and the fresh result reports the compiler check passing at that version (AC-SET-3, AC-SET-6, AC-SET-19, AC-SET-23)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			path, miseDir := pathWithStatefulStubNodeAndMise("v24.9.9", "7.0.2")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			revision, err := gitrepo.ResolveBaselineRevision(repo)
			Expect(err).NotTo(HaveOccurred())
			before, err := projectcheck.Run(repo, revision, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(before.Checks.Compiler.State).To(Equal(projectreadiness.Fail), "sanity: the fixture must start without a usable compiler")
			Expect(before.Checks.Compiler.Code).To(Equal(projectreadiness.GapTypescriptCompilerMissing))

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			transcript := readStderr()
			Expect(exitCode).To(Equal(0), "stderr: %s", transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue(), "a confirmed selection must invoke `mise install`")

			Expect(transcript).To(ContainSubstring("installed TypeScript 7.0.2 via mise_project; rerun readiness reports compiler check pass (version=7.0.2)"), "transcript: %s", transcript)
		})
	})

	When("the project mise.toml already declares the frozen TypeScript version but `mise install` itself fails, and the user selects mise_project and confirms", func() {
		It("exits 2 with empty stdout, a transcript naming the version and scope, and no repository or mise-store mutation from Coach itself (AC-SET-7)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			path, miseDir := pathWithFailingInstallStubNodeAndMise("v24.9.9")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			statusBefore := gitStatusPorcelain(repo)

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			transcript := readStderr()
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", readStdout(), transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue(), "a confirmed selection must still invoke `mise install`, even though it fails")

			Expect(transcript).To(MatchRegexp(`mise install failed; mise's install store may now contain a partial or failed install of TypeScript 7\.0\.2 under the mise_project scope`), "the failure message must name the actual version and scope that may have been partially installed, not an empty string, transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Coach does not attempt to clean this up"))

			Expect(gitStatusPorcelain(repo)).To(Equal(statusBefore), "a failed install must never leave Coach itself having mutated the repository (no rollback is attempted, but none should be needed)")
		})
	})

	When("the project mise.toml already declares the frozen TypeScript version and `mise install` itself exits 0, but the freshly-installed compiler never becomes locatable/eligible, and the user selects mise_project and confirms", func() {
		It("exits 2 with empty stdout and a transcript that names the real verification failure, never the wrong 'mise install failed' wording (coach#328 Task 5 integration repair, Finding 2)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			path, miseDir := pathWithIneligibleInstallStubNodeAndMise("v24.9.9")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			transcript := readStderr()
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", readStdout(), transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue(), "a confirmed selection must still invoke `mise install`, even though verification later fails")

			Expect(transcript).To(ContainSubstring("mise install exited 0 but the installed TypeScript 7.0.2 is not eligible (absent); expected the native platform package "+tstoolchain.NativeTypescriptPackageName()+" alongside it -- Coach does not attempt to repair or clean this up."), "transcript: %s", transcript)
			Expect(transcript).NotTo(ContainSubstring("mise install failed"), "a subprocess that exited 0 must never be described as having failed, transcript: %s", transcript)
		})
	})

	When("mise install could never even be started because its own private working directory could not be created", func() {
		It("reports the install could not even be started, naming the insulation-failure gap code, distinctly from a subprocess that actually ran and failed (coach#328 Task 5 integration repair, Finding 5)", func() {
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			result := tssetup.PrepareCompilerMiseResult{
				Trusted: true,
				Code:    projectreadiness.GapPackageManagerConfigUnverifiable,
			}

			exitCode := reportPrepareCompilerMiseResult(result, stderrFile)

			transcript := readStderr()
			Expect(exitCode).To(Equal(2), "stderr: %s", transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(transcript).To(ContainSubstring("mise install could not even be started (package_manager_config_unverifiable)."), "transcript: %s", transcript)
			Expect(transcript).NotTo(ContainSubstring("mise install failed"), "an install that never started must never be described as having failed, transcript: %s", transcript)
		})
	})

	When("the project mise scope is untrusted (a hazardous mise.toml) so only mise_global is offered, and the user selects it and confirms", func() {
		It("installs via the global scope, reruns readiness, and the fresh result reports the compiler check passing at that version (AC-SET-3, AC-SET-6, AC-SET-19, AC-SET-23)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[hooks]\npostinstall = \"echo pwned\"\n")
			miseDir := writeStatefulStubMiseScriptGlobalAware("7.0.2")
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			revision, err := gitrepo.ResolveBaselineRevision(repo)
			Expect(err).NotTo(HaveOccurred())
			before, err := projectcheck.Run(repo, revision, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(before.Checks.Compiler.State).To(Equal(projectreadiness.Fail), "sanity: the fixture must start without a usable compiler")

			stdin := authoringStdin("mise_global\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			transcript := readStderr()

			Expect(transcript).To(ContainSubstring("  - mise_global"), "transcript: %s", transcript)
			Expect(transcript).NotTo(ContainSubstring("  - mise_project"), "the hazardous project mise.toml must withhold mise_project from the offered choices, transcript: %s", transcript)

			Expect(exitCode).To(Equal(0), "stderr: %s", transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue(), "a confirmed selection must invoke `mise install`")

			Expect(transcript).To(ContainSubstring("installed TypeScript 7.0.2 via mise_global; rerun readiness reports compiler check pass (version=7.0.2)"), "transcript: %s", transcript)
		})
	})

	When("the caller supplies a context deadline shorter than mise's own five-minute install timeout, and `mise install` runs long enough to exceed it", func() {
		It("cuts the install off at that shorter deadline: attempted but never observed, well before the stub's own sleep would otherwise finish", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			miseDir, err := os.MkdirTemp("", "coach-acceptance-slowinstallmise-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, miseDir)
			script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %q\n"+
				"if [ \"$1\" = \"--version\" ]; then echo \"2026.9.5 linux-x64 (2026-09-10)\"; exit 0; fi\n"+
				"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo \"[]\"; exit 0; fi\n"+
				"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"get\" ]; then exit 1; fi\n"+
				"if [ \"$1\" = \"install\" ]; then exec sleep 30; fi\n"+
				"exit 1\n", filepath.Join(miseDir, stubMiseInvocationLog))
			Expect(os.WriteFile(filepath.Join(miseDir, "mise"), []byte(script), 0o755)).To(Succeed())
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			revision, err := gitrepo.ResolveBaselineRevision(repo)
			Expect(err).NotTo(HaveOccurred())
			readiness, err := projectcheck.Run(repo, revision, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(readiness.Checks.Compiler.Code).To(Equal(projectreadiness.GapTypescriptCompilerMissing), "sanity: the fixture must start without a usable compiler")

			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			var transcript bytes.Buffer

			result := tssetup.RunPrepareCompilerMiseSetup(ctx, repo, revision, "", readiness, stdin, &transcript)
			elapsed := time.Since(start)

			Expect(elapsed).To(BeNumerically(">=", 2*time.Second), "the install must run at least as long as the supplied context deadline, not fail some unrelated, faster way, got elapsed=%s transcript=%s", elapsed, transcript.String())
			Expect(elapsed).To(BeNumerically("<", 15*time.Second), "a 2s context deadline must cut the install off well before the stub's 30s sleep would otherwise finish, got elapsed=%s transcript=%s", elapsed, transcript.String())
			Expect(result.Trusted).To(BeTrue(), "%+v", result)
			Expect(result.Attempted).To(BeTrue(), "the install subprocess must have actually started: %+v", result)
			Expect(result.Succeeded).To(BeFalse(), "a deadline-cut install must never be reported as succeeded: %+v", result)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue())
		})
	})
})
