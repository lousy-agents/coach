package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			writeWorktreeFile(repo, "node_modules/typescript/package.json", `{"name":"typescript","version":"5.4.0"}`+"\n")

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
			Expect(nextActionKinds(doc)).To(ContainElement("prepare_compiler"))

			action, _ := nextActionOfKind(doc, "prepare_compiler")
			Expect(action.Kind).To(Equal("prepare_compiler"))
			Expect(action.Executable).To(BeTrue())
			Expect(action.RuntimeKind).To(BeEmpty(), "prepare_compiler carries no runtime_kind field")
			Expect(action.Supported).To(Equal([]string{"7.0.2"}))
			Expect(action.FoundVersion).To(Equal("5.4.0"))
			Expect(action.Detail).To(BeEmpty(), "prepare_compiler carries no detail field")
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
			repo := newTempGitRepo()
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"5.4.0"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, _ := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "the compiler installed at the project origin is its candidate whatever the manifest declares, got state=%s code=%s stdout=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, stdout)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(doc.Status).To(Equal("ready_with_limits"))
			Expect(gapCodes(doc)).NotTo(ContainElement("compiler_declaration_mismatch"))
			Expect(doc.Warnings).To(ContainElement(HaveField("Code", "compiler_declaration_mismatch")))
			var warning struct {
				DeclaredVersion   string
				FoundVersion      string
				DeclarationOrigin string
			}
			for _, w := range doc.Warnings {
				if w.Code == "compiler_declaration_mismatch" {
					warning.DeclaredVersion = w.DeclaredVersion
					warning.FoundVersion = w.FoundVersion
					warning.DeclarationOrigin = w.DeclarationOrigin
				}
			}
			Expect(warning.DeclaredVersion).To(Equal("5.4.0"), "warning must name the stale exact pin, got %+v stdout=%s", warning, stdout)
			Expect(warning.FoundVersion).To(Equal("7.0.2"))
			Expect(warning.DeclarationOrigin).To(Equal("manifest"))
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
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			sentinel := filepath.Join(repo, "mise-exec-side-effect")
			writeWorktreeFile(repo, "mise.toml", fmt.Sprintf("[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[env]\nSIDE_EFFECT = \"{{ exec(command='touch %s') }}\"\n", sentinel))

			path, miseDir := pathWithStubNodeAndMise("v24.9.9", "7.0.2")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).NotTo(BeEmpty())

			_, statErr := os.Stat(sentinel)
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "mise exec template must not run during the fit check")
			for _, cwd := range readStubMiseCwds(miseDir) {
				Expect(cwd).NotTo(Equal(repo), "mise probes must not run with the analyzed repository as cwd, got %q", cwd)
			}
		})
	})
})
