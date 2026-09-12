package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const twoRootPolicyJSON = `{"schema_version":"1","roots":["apps/web","apps/api"]}` + "\n"

const singleRootPolicyJSON = `{"schema_version":"1","roots":["."]}` + "\n"

func checkProjectBothFormats(repo, path string, extraArgs ...string) (readinessResultDoc, string) {
	base := append([]string{"--baseline", "--check-project", "--project-language", "typescript"}, extraArgs...)

	jsonArgs := append(append([]string{}, base...), "--format", "json")
	jsonStdout, jsonStderr, jsonExit := runCoachCheckProjectEnv(repo, path, jsonArgs...)
	ExpectWithOffset(1, jsonExit).To(Equal(0), "stderr: %s", jsonStderr)
	var doc readinessResultDoc
	ExpectWithOffset(1, json.Unmarshal(jsonStdout, &doc)).To(Succeed(), "stdout: %s", jsonStdout)

	textArgs := append([]string{}, base...)
	textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, textArgs...)
	ExpectWithOffset(1, textExit).To(Equal(0), "stderr: %s", textStderr)

	return doc, string(textStdout)
}

func declarationMismatchWarning(doc readinessResultDoc) (declared, found, origin string, present bool) {
	for _, warning := range doc.Warnings {
		if warning.Code == "compiler_declaration_mismatch" {
			return warning.DeclaredVersion, warning.FoundVersion, warning.DeclarationOrigin, true
		}
	}
	return "", "", "", false
}

func warningCodes(doc readinessResultDoc) []string {
	codes := make([]string, 0, len(doc.Warnings))
	for _, warning := range doc.Warnings {
		codes = append(codes, warning.Code)
	}
	return codes
}

func rootFindingPairs(doc readinessResultDoc) []string {
	pairs := make([]string, 0, len(doc.Checks.Compiler.RootFindings))
	for _, finding := range doc.Checks.Compiler.RootFindings {
		if finding.Version == "" {
			pairs = append(pairs, finding.Root)
			continue
		}
		pairs = append(pairs, finding.Root+"@"+finding.Version)
	}
	return pairs
}

// defaultStubMiseToolVersion is a supported mise-tool-version response
// (matching miseToolSupportedCalverYear), used as the default so this
// file's specs that predate mise-tool-version/config-hazard gating
// (SA-280-015/SA-280-045) keep resolving through mise exactly as before.
const defaultStubMiseToolVersion = "2026.9.5 linux-x64 (2026-09-10)"

// writeVersionedStubMiseScript extends writeStubMiseScript's contract
// (project_readiness_acceptance_test.go) with distinct, independently
// controllable responses for `mise --version` and `mise config ls -J`.
// That shared helper cannot do this itself: it echoes the same miseVersion
// argument for every invocation except `where`, which represents a
// TypeScript version in every spec that already depends on it, not a
// mise-tool version -- reusing it here would make every existing mise-origin
// spec in this file collide with the new mise-tool-version gate.
//
// tsVersion seeds the `where`-fixture on disk (a real installed compiler at
// that version) but is otherwise unused unless whereVersion is requested;
// globalConfigVersion is what `config get tools.npm:typescript -g` reports
// (empty means "not configured", matching detectGlobalMiseTypescriptVersion's
// contract); toolVersionOutput is what `--version` reports (empty means the
// probe fails, modeling an undetectable mise-tool version); configLsJSON is
// the raw `config ls -J` response.
func writeVersionedStubMiseScript(tsVersion, toolVersionOutput, configLsJSON, globalConfigVersion string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubmise-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	installDir := filepath.Join(dir, "install")
	if tsVersion != "" {
		Expect(os.MkdirAll(filepath.Join(installDir, "node_modules", "typescript"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(installDir, "node_modules", "typescript", "package.json"), []byte(fmt.Sprintf(`{"name":"typescript","version":%q}`+"\n", tsVersion)), 0o644)).To(Succeed())
		nativeUnscoped := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
		nativeDir := filepath.Join(installDir, "node_modules", "@typescript", nativeUnscoped)
		Expect(os.MkdirAll(nativeDir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(nativeDir, "package.json"), []byte(fmt.Sprintf(`{"name":%q,"version":%q}`+"\n", "@typescript/"+nativeUnscoped, tsVersion)), 0o644)).To(Succeed())
	}

	versionBranch := "exit 1"
	if toolVersionOutput != "" {
		versionBranch = fmt.Sprintf("echo %q; exit 0", toolVersionOutput)
	}
	configGetBranch := "exit 1"
	if globalConfigVersion != "" {
		configGetBranch = fmt.Sprintf("echo %q; exit 0", globalConfigVersion)
	}
	script := fmt.Sprintf(
		"#!/bin/sh\necho \"$PWD\" >> %q\necho \"$@\" >> %q\n"+
			"if [ \"$1\" = \"--version\" ]; then %s; fi\n"+
			"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo %q; exit 0; fi\n"+
			"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"get\" ]; then %s; fi\n"+
			"if [ \"$1\" = \"where\" ]; then echo %q; exit 0; fi\n"+
			"exit 1\n",
		filepath.Join(dir, stubMiseCwdLog), filepath.Join(dir, stubMiseInvocationLog),
		versionBranch, configLsJSON, configGetBranch, installDir,
	)
	Expect(os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755)).To(Succeed())
	return dir
}

func pathWithVersionedStubMise(nodeVersion, tsVersion, toolVersionOutput, configLsJSON, globalConfigVersion string) (path, miseDir string) {
	miseDir = writeVersionedStubMiseScript(tsVersion, toolVersionOutput, configLsJSON, globalConfigVersion)
	path = writeStubNodeScript(nodeVersion) + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingExecutables("node", "npm", "mise")
	return path, miseDir
}

// pathWithStubMiseDefaultTool is a drop-in replacement for this file's
// former use of the shared pathWithStubNodeAndMise helper: it answers
// `mise --version` with a supported version and `mise config ls -J` with an
// empty (hazard-free) config list, and otherwise reports tsVersion for both
// `config get` and `where`, exactly like the shared helper did.
func pathWithStubMiseDefaultTool(nodeVersion, tsVersion string) (path, miseDir string) {
	return pathWithVersionedStubMise(nodeVersion, tsVersion, defaultStubMiseToolVersion, "[]", tsVersion)
}

func gapEntries(doc readinessResultDoc) []string {
	entries := make([]string, len(doc.Gaps))
	for i, g := range doc.Gaps {
		if g.PackageManagerKind == "" {
			entries[i] = g.Code
			continue
		}
		entries[i] = g.Code + ":" + g.PackageManagerKind
	}
	return entries
}

func prepareCompilerChoices(doc readinessResultDoc) ([]string, bool) {
	for _, action := range doc.NextActions {
		if action.Kind == "prepare_compiler" {
			return action.Choices, true
		}
	}
	return nil, false
}

func commitMixedRootFixture(repo, declaredVersion string) {
	commitFile(repo, "project.json", twoRootPolicyJSON)
	commitFile(repo, "apps/web/src/index.ts", "export const web = 1;\n")
	commitFile(repo, "apps/api/package.json", `{"name":"api","version":"1.0.0","devDependencies":{"typescript":"`+declaredVersion+`"}}`+"\n")
	commitFile(repo, "apps/api/src/index.ts", "export const api = 1;\n")
	writeInstalledTypescriptUnder(repo, "apps/api", declaredVersion)
}

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: compiler origin aggregation (SA-280-043/044)", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
	})

	When("one selected root has no manifest context while another declares and installs a different version", func() {
		BeforeEach(func() {
			commitMixedRootFixture(repo, "5.9.3")
		})

		When("global mise supplies a supported compiler", func() {
			It("passes from that origin, warning that a root's manifest declares another version", func() {
				path, miseDir := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "a root with no manifest context makes the project origin unavailable and must never block the mise origins, got state=%s code=%s root_findings=%v", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, rootFindingPairs(doc))
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(gapCodes(doc)).To(BeEmpty())
				Expect(doc.Status).To(Equal("ready_with_limits"))

				declared, found, origin, present := declarationMismatchWarning(doc)
				Expect(present).To(BeTrue(), "a winning non-project origin plus a differing manifest declaration must warn, got warnings=%v", warningCodes(doc))
				Expect(declared).To(Equal("5.9.3"))
				Expect(found).To(Equal("7.0.2"))
				Expect(origin).To(Equal("manifest"))

				Expect(text).To(ContainSubstring("compiler: pass (compiler_declaration_mismatch) version=7.0.2"))
				Expect(text).To(ContainSubstring("compiler_declaration_mismatch (declared_version=5.9.3 found_version=7.0.2 declaration_origin=manifest root=apps/api)"))

				Expect(strings.Join(readStubMiseInvocations(miseDir), "\n")).To(ContainSubstring("config get tools.npm:typescript -g"), "the winning candidate must come from the read-only global mise probe")
			})
		})

		When("no mise origin is configured", func() {
			It("reports typescript_version_conflict naming every selected root's finding", func() {
				path := pathWithStubNode("v24.9.9")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("fail"))
				Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_conflict"))
				Expect(doc.Checks.Compiler.ExpectedVersion).To(BeEmpty(), "conflict omits expected_version")
				Expect(doc.Checks.Compiler.FoundVersion).To(BeEmpty(), "conflict omits found_version")
				Expect(rootFindingPairs(doc)).To(ConsistOf("apps/web", "apps/api@5.9.3"))
				Expect(gapCodes(doc)).To(ContainElement("typescript_version_conflict"))
				Expect(nextActionKinds(doc)).To(ContainElement("prepare_compiler"))

				Expect(text).To(ContainSubstring("root_findings=apps/web,apps/api@5.9.3"))
			})
		})
	})

	When("the selected root declares the supported version and installs one outside the set", func() {
		BeforeEach(func() {
			commitFile(repo, "project.json", singleRootPolicyJSON)
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "5.9.3")
		})

		When("no mise origin is configured", func() {
			It("reports typescript_version_mismatch naming the installed version and the supported set", func() {
				path := pathWithStubNode("v24.9.9")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("fail"))
				Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_mismatch"))
				Expect(doc.Checks.Compiler.ExpectedVersion).To(Equal("7.0.2"))
				Expect(doc.Checks.Compiler.FoundVersion).To(Equal("5.9.3"), "the installed compiler governs, never the declaration")
				Expect(doc.Checks.Compiler.SupportedVersions).To(Equal([]string{"7.0.2"}))

				Expect(text).To(ContainSubstring("compiler: fail (typescript_version_mismatch)"))
				Expect(text).To(ContainSubstring("supported_versions=7.0.2"))
			})
		})

		When("project mise supplies a supported compiler", func() {
			It("passes from that origin: an unsupported candidate never stops a lower-precedence origin", func() {
				commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

				path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "an unsupported project candidate must not be terminal, got state=%s code=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code)
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(gapCodes(doc)).To(BeEmpty())
				Expect(warningCodes(doc)).To(BeEmpty(), "a declaration equal to the winner's probed version carries no warning")
				Expect(doc.Status).To(Equal("ready"))

				Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
				Expect(text).NotTo(ContainSubstring("typescript_version_mismatch"))
			})
		})
	})

	When("the selected root declares a version it never installed and project mise supplies a supported compiler", func() {
		BeforeEach(func() {
			commitFile(repo, "project.json", singleRootPolicyJSON)
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
		})

		When("the declaration names a version outside the supported set", func() {
			It("passes from the mise origin, warning that the manifest declares another version", func() {
				commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"5.9.3"}}`+"\n")

				path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "a declared-but-not-installed pin makes the project origin unavailable; it never reports typescript_compiler_missing without evaluating mise, got state=%s code=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code)
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(gapCodes(doc)).To(BeEmpty())

				declared, found, origin, present := declarationMismatchWarning(doc)
				Expect(present).To(BeTrue(), "got warnings=%v", warningCodes(doc))
				Expect(declared).To(Equal("5.9.3"))
				Expect(found).To(Equal("7.0.2"))
				Expect(origin).To(Equal("manifest"))

				Expect(text).To(ContainSubstring("compiler: pass (compiler_declaration_mismatch) version=7.0.2"))
				Expect(text).To(ContainSubstring("compiler_declaration_mismatch (declared_version=5.9.3 found_version=7.0.2 declaration_origin=manifest root=.)"))
			})
		})

		When("the declaration names exactly the version mise supplies", func() {
			It("passes from the mise origin with no warning", func() {
				commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")

				path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "got state=%s code=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code)
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(gapCodes(doc)).To(BeEmpty())
				Expect(warningCodes(doc)).To(BeEmpty(), "a declaration equal to the winner's version, merely uninstalled at the project origin, is not a limit")
				Expect(doc.Status).To(Equal("ready"))

				Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
				Expect(text).NotTo(ContainSubstring("compiler_declaration_mismatch"))
			})
		})
	})

	When("the selected root's manifest cannot be read", func() {
		BeforeEach(func() {
			commitFile(repo, "project.json", singleRootPolicyJSON)
		})

		When("it is not valid JSON and project mise supplies a supported compiler", func() {
			It("passes from that origin: an unreadable manifest never stops a lower-precedence origin", func() {
				commitFile(repo, "package.json", `{"name":"example",`+"\n")
				commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

				path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "got state=%s code=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code)
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(gapCodes(doc)).To(BeEmpty())
				Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
			})
		})

		When("the process cannot open it and project mise supplies a supported compiler", func() {
			It("passes from that origin rather than reporting a compiler gap", func() {
				if os.Geteuid() == 0 {
					Skip("running as root defeats a permission-denied manifest fixture")
				}
				commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
				commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
				Expect(os.Chmod(filepath.Join(repo, "package.json"), 0o000)).To(Succeed())
				DeferCleanup(func() { _ = os.Chmod(filepath.Join(repo, "package.json"), 0o644) })

				path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "got state=%s code=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code)
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
			})
		})

		When("no mise origin is configured", func() {
			It("reports typescript_compiler_missing naming the unreadable class, distinct from an absent or unconfigured origin", func() {
				commitFile(repo, "package.json", `{"name":"example",`+"\n")

				path := pathWithStubNode("v24.9.9")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
				Expect(text).To(ContainSubstring("origins=project:unreadable,mise_project:unconfigured,mise_global:unconfigured"),
					"an unreadable manifest must be named as such, not collapsed into absent or unconfigured, got %s", text)
			})
		})
	})

	When("the approved compiler is installed without its native platform package", func() {
		BeforeEach(func() {
			commitFile(repo, "project.json", singleRootPolicyJSON)
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescriptCompilerOnly(repo, "7.0.2")
		})

		When("no mise origin is configured", func() {
			It("reports typescript_compiler_missing with the compiler's found_version and the native package in detail", func() {
				path := pathWithStubNode("v24.9.9")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("fail"))
				Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
				Expect(doc.Checks.Compiler.FoundVersion).To(Equal("7.0.2"))

				Expect(text).To(ContainSubstring("detail=" + nativeTypescriptPackageLookupName()))
				Expect(text).To(ContainSubstring("origins=project:native_invalid"))
			})
		})

		When("project mise supplies a complete compiler", func() {
			It("passes from that origin: a native-invalid candidate never stops a lower-precedence origin", func() {
				commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

				path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "got state=%s code=%s detail=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, text)
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(gapCodes(doc)).To(BeEmpty())
				Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
			})
		})
	})

	When("every origin expects a compiler that is installed nowhere", func() {
		It("reports typescript_compiler_missing with no found_version, naming every origin's class", func() {
			commitFile(repo, "project.json", singleRootPolicyJSON)
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path := pathWithStubNode("v24.9.9")

			doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(doc.Checks.Compiler.ExpectedVersion).To(Equal("7.0.2"))
			Expect(doc.Checks.Compiler.FoundVersion).To(BeEmpty(), "nothing was probed on disk, so there is no found_version to report")
			Expect(doc.Checks.Compiler.RootFindings).To(BeEmpty(), "root_findings accompanies only typescript_version_conflict")

			// mise_project is "unconfigured", not "absent": with mise itself
			// absent from PATH, the mise-tool-version gate (SA-280-015)
			// refuses to trust reading mise.toml's declared version at all,
			// rather than reading it and then discovering it cannot be
			// located.
			Expect(text).To(ContainSubstring("origins=project:absent,mise_project:unconfigured,mise_global:unconfigured"), "remediation text must name each origin's class, got %s", text)
			Expect(text).NotTo(ContainSubstring(repo+string(os.PathSeparator)), "remediation must never print a filesystem path, got %s", text)
		})
	})

	When("no selected root resolves a manifest candidate and project mise pins an installed out-of-set version", func() {
		It("reports typescript_version_mismatch naming that origin's installed version", func() {
			commitFile(repo, "project.json", singleRootPolicyJSON)
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"9.9.9\"\n")

			path, _ := pathWithStubMiseDefaultTool("v24.9.9", "9.9.9")

			doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_mismatch"), "an unsupported candidate at a mise origin is still an unsupported candidate, got code=%s", doc.Checks.Compiler.Code)
			Expect(doc.Checks.Compiler.FoundVersion).To(Equal("9.9.9"))
			Expect(doc.Checks.Compiler.ExpectedVersion).To(Equal("7.0.2"))
			Expect(doc.Checks.Compiler.SupportedVersions).To(Equal([]string{"7.0.2"}))
			Expect(text).To(ContainSubstring("found_version=9.9.9"))
		})
	})

	When("one project mise [tools] table pins two exact versions under a policy selecting two nested roots", func() {
		It("reports typescript_version_conflict naming each selected root, never a root the policy did not select", func() {
			commitFile(repo, "project.json", twoRootPolicyJSON)
			commitFile(repo, "apps/web/package.json", `{"name":"web","version":"1.0.0"}`+"\n")
			commitFile(repo, "apps/api/src/index.ts", "export const api = 1;\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = [\"7.0.2\", \"5.4.0\"]\n")

			path := pathWithStubNode("v24.9.9")

			doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_conflict"))
			Expect(rootFindingPairs(doc)).To(ConsistOf("apps/web", "apps/api"),
				"root_findings is one entry per selected root, never a literal \".\" the policy did not select, got %v", rootFindingPairs(doc))
			Expect(text).NotTo(ContainSubstring("root_findings=."), "text must not name an unselected root, got %s", text)
		})
	})

	When("two selected roots install disagreeing compiler versions and project mise also supplies a supported compiler", func() {
		It("reports typescript_version_conflict: disagreement among installed candidates never yields to mise", func() {
			commitFile(repo, "project.json", twoRootPolicyJSON)
			commitFile(repo, "apps/web/package.json", `{"name":"web","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			commitFile(repo, "apps/api/package.json", `{"name":"api","version":"1.0.0","devDependencies":{"typescript":"5.4.0"}}`+"\n")
			writeInstalledTypescriptUnder(repo, "apps/web", "7.0.2")
			writeInstalledTypescriptUnder(repo, "apps/api", "5.4.0")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, miseDir := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

			doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("fail"), "mise matching one root must not certify the pair, got state=%s code=%s version=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code, doc.Checks.Compiler.Version)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_conflict"))
			Expect(rootFindingPairs(doc)).To(ConsistOf("apps/web@7.0.2", "apps/api@5.4.0"))
			Expect(gapCodes(doc)).To(ContainElement("typescript_version_conflict"))
			Expect(text).To(ContainSubstring("root_findings=apps/web@7.0.2,apps/api@5.4.0"))
			Expect(text).NotTo(ContainSubstring("compiler: pass"))
			_, logErr := os.Stat(filepath.Join(miseDir, stubMiseInvocationLog))
			Expect(os.IsNotExist(logErr)).To(BeTrue(),
				"project-origin conflict must not evaluate mise at all; a fall-through that then discarded mise would still look like conflict after probing it")
		})
	})

	When("two selected roots each declare a version other than the one mise supplies", func() {
		It("warns once per disagreeing root, in the policy's roots order, each naming its own root", func() {
			commitFile(repo, "project.json", twoRootPolicyJSON)
			commitFile(repo, "apps/web/package.json", `{"name":"web","version":"1.0.0","devDependencies":{"typescript":"5.9.3"}}`+"\n")
			commitFile(repo, "apps/api/package.json", `{"name":"api","version":"1.0.0","devDependencies":{"typescript":"^7.0.2"}}`+"\n")

			path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

			doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "got code=%s", doc.Checks.Compiler.Code)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))

			var declared []string
			var roots []string
			for _, warning := range doc.Warnings {
				if warning.Code != "compiler_declaration_mismatch" {
					continue
				}
				declared = append(declared, warning.DeclaredVersion)
				roots = append(roots, warning.Root)
				Expect(warning.FoundVersion).To(Equal("7.0.2"), "found_version is the installed version the scan will use")
				Expect(warning.DeclarationOrigin).To(Equal("manifest"))
			}
			Expect(declared).To(Equal([]string{"5.9.3", "^7.0.2"}), "one entry per disagreeing root, in the policy's roots order, got warnings=%+v", doc.Warnings)
			Expect(roots).To(Equal([]string{"apps/web", "apps/api"}), "each entry names its own root, got %+v", doc.Warnings)
			Expect(text).To(ContainSubstring("root=apps/web"))
			Expect(text).To(ContainSubstring("root=apps/api"))
		})
	})

	DescribeTable("checks.compiler carries only the frozen fields, whatever the outcome",
		func(fixture func(repo string), miseVersion string, wantCode string) {
			commitFile(repo, "project.json", singleRootPolicyJSON)
			fixture(repo)

			path := pathWithStubNode("v24.9.9")
			if miseVersion != "" {
				path, _ = pathWithStubMiseDefaultTool("v24.9.9", miseVersion)
			}

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var raw struct {
				Checks struct {
					Compiler map[string]json.RawMessage `json:"compiler"`
				} `json:"checks"`
			}
			Expect(json.Unmarshal(stdout, &raw)).To(Succeed(), "stdout: %s", stdout)

			var code string
			if encoded, ok := raw.Checks.Compiler["code"]; ok {
				Expect(json.Unmarshal(encoded, &code)).To(Succeed())
			}
			Expect(code).To(Equal(wantCode), "fixture must reach the intended outcome for this guard to mean anything, got %v", raw.Checks.Compiler)

			frozen := map[string]bool{
				"state": true, "code": true, "version": true, "expected_version": true,
				"found_version": true, "supported_versions": true, "root_findings": true, "detail": true,
			}
			for field := range raw.Checks.Compiler {
				Expect(frozen).To(HaveKey(field), "checks.compiler carries only the frozen fields, got %v", raw.Checks.Compiler)
			}
		},
		Entry("pass from mise with a declaration warning", func(repo string) {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"5.9.3"}}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
		}, "7.0.2", "compiler_declaration_mismatch"),
		Entry("typescript_version_mismatch", func(repo string) {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "5.9.3")
		}, "", "typescript_version_mismatch"),
		Entry("typescript_compiler_missing, where the origin classes are populated", func(repo string) {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
		}, "", "typescript_compiler_missing"),
		Entry("typescript_version_conflict", func(repo string) {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","dependencies":{"typescript":"7.0.2"},"devDependencies":{"typescript":"5.4.0"}}`+"\n")
		}, "", "typescript_version_conflict"),
	)
})

var _ = Describe("coach codesignal --baseline --project-language typescript: the scan resolves its compiler through the same aggregation (SA-280-043/044)", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
		commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
		commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
		commitFile(repo, "project.json", goLayerPolicyConfigJSON)
	})

	When("the table reports a compiler pass from a mise origin the project origin could not supply", func() {
		It("never refuses the scan: a readiness pass means the scan can load that same compiler", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"5.9.3"}}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

			checkStdout, checkStderr, checkExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(checkExit).To(Equal(0), "stderr: %s", checkStderr)
			var doc readinessResultDoc
			Expect(json.Unmarshal(checkStdout, &doc)).To(Succeed())
			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "this fixture is only meaningful while readiness passes, got code=%s", doc.Checks.Compiler.Code)

			_, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).NotTo(Equal(2), "a passing readiness compiler must never be an unresolved scan-time compiler, stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("typescript_compiler_missing"))
			Expect(string(stderr)).NotTo(ContainSubstring("typescript_version_mismatch"))
			Expect(string(stderr)).NotTo(ContainSubstring("typescript_version_conflict"))
		})
	})

	When("the table reports typescript_compiler_missing", func() {
		It("refuses the scan with exit 2 and that same gap code on one stderr line, with no root finding to name", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
		})
	})

	When("the table reports typescript_version_mismatch for a compiler no selected root resolved", func() {
		It("refuses the scan naming that gap with no root segment, never attributing it to a root that resolved nothing", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"5.9.3\"\n")

			path, _ := pathWithStubMiseDefaultTool("v24.9.9", "5.9.3")

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_version_mismatch: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"),
				"a mismatch a mise origin caused must not attribute the unsupported compiler to a selected root that resolved nothing, got %q", stderr)
		})
	})
})

var _ = Describe("coach codesignal --baseline --check-project: read-only mise probe confinement", func() {
	When("a read-only mise probe runs during --check-project", func() {
		It("uses a private per-invocation working directory, not a fixed shared path any local user could plant configuration in", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", singleRootPolicyJSON)
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, miseDir := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

			_, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			shared := filepath.Join(os.TempDir(), "coach-mise-probe")
			cwds := readStubMiseCwds(miseDir)
			Expect(cwds).NotTo(BeEmpty())
			for _, cwd := range cwds {
				Expect(cwd).NotTo(Equal(repo), "a probe must never run with the analyzed repository as cwd, got %q", cwd)
				Expect(cwd).NotTo(Equal(shared), "a probe must not run in a predictable shared directory another local user can pre-create, got %q", cwd)
				Expect(cwd).NotTo(BeADirectory(), "the private probe directory must be removed after the probe, %q still exists", cwd)
			}
		})
	})
})

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: declaration versus installed compiler (D4)", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
		commitFile(repo, "project.json", singleRootPolicyJSON)
	})

	When("the manifest declares a range and a supported compiler is installed beside it", func() {
		BeforeEach(func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"^7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
		})

		When("no mise origin is configured", func() {
			It("passes from the project origin with no warning, since the installed compiler is the candidate and a range is never warned about", func() {
				path := pathWithStubNode("v24.9.9")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "the installed compiler is the project origin's candidate whatever the manifest declares, got state=%s code=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code)
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(doc.Checks.Runtime.State).To(Equal("pass"))
				Expect(doc.Checks.Runtime.Code).To(BeEmpty(), "Node 24 is a supported major and must carry no code or warning")
				Expect(doc.Checks.Node.Code).To(BeEmpty())
				Expect(warningCodes(doc)).To(BeEmpty(), "a range declaration is never warned about when the project origin wins, got %v", warningCodes(doc))
				Expect(gapCodes(doc)).To(BeEmpty())
				Expect(doc.Status).To(Equal("ready"))

				Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
				Expect(text).NotTo(ContainSubstring("compiler_declaration_mismatch"))
			})
		})

		When("project mise pins the same supported version", func() {
			It("still passes from the project origin with no warning, since the project origin outranks mise and its range declaration is not warned about", func() {
				commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

				path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"))
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(warningCodes(doc)).To(BeEmpty(), "got %v", warningCodes(doc))
				Expect(doc.Status).To(Equal("ready"))

				Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
				Expect(text).NotTo(ContainSubstring("compiler_declaration_mismatch"))
			})
		})
	})

	When("the manifest declares a stale exact version beside an installed supported compiler and no mise origin is configured", func() {
		It("passes from the project origin and warns that the root's manifest declares another version", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"5.4.0"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")

			path := pathWithStubNode("v24.9.9")

			doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "an out-of-set declaration governs setup choices only; the installed 7.0.2 is the candidate, got state=%s code=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(doc.Checks.Runtime.State).To(Equal("pass"))
			Expect(doc.Checks.Runtime.Code).To(BeEmpty(), "Node 24 is a supported major and must carry no code or warning, even while the compiler warns")
			Expect(doc.Checks.Node.Code).To(BeEmpty())
			Expect(doc.Status).To(Equal("ready_with_limits"))

			declared, found, origin, present := declarationMismatchWarning(doc)
			Expect(present).To(BeTrue(), "an exact declaration differing from the installed version is a stale pin and must warn, got warnings=%v", warningCodes(doc))
			Expect(declared).To(Equal("5.4.0"))
			Expect(found).To(Equal("7.0.2"))
			Expect(origin).To(Equal("manifest"))
			Expect(len(warningCodes(doc))).To(Equal(1), "only the compiler warning may be present -- a supported Node major never contributes one, got %v", warningCodes(doc))

			Expect(text).To(ContainSubstring("compiler: pass (compiler_declaration_mismatch) version=7.0.2"))
			Expect(text).To(ContainSubstring("compiler_declaration_mismatch (declared_version=5.4.0 found_version=7.0.2 declaration_origin=manifest root=.)"))
		})
	})

	When("the manifest declares a range and the compiler installed beside it is outside the supported set", func() {
		BeforeEach(func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"^5.4.0"}}`+"\n")
			writeInstalledTypescript(repo, "5.4.0")
		})

		When("no mise origin is configured", func() {
			It("reports typescript_version_mismatch naming the installed version, since the installed compiler is the candidate and it is out of set", func() {
				path := pathWithStubNode("v24.9.9")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("fail"))
				Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_mismatch"), "got code=%s", doc.Checks.Compiler.Code)
				Expect(doc.Checks.Compiler.ExpectedVersion).To(Equal("7.0.2"))
				Expect(doc.Checks.Compiler.FoundVersion).To(Equal("5.4.0"))
				Expect(doc.Checks.Compiler.SupportedVersions).To(Equal([]string{"7.0.2"}))

				Expect(text).To(ContainSubstring("found_version=5.4.0"))
			})
		})

		When("global mise supplies a supported compiler", func() {
			It("passes from the mise origin and warns that the root's manifest declares another version", func() {
				path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "an unsupported project candidate never stops a lower-precedence origin, got state=%s code=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code)
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(doc.Status).To(Equal("ready_with_limits"))

				declared, found, _, present := declarationMismatchWarning(doc)
				Expect(present).To(BeTrue(), "a winning non-project origin warns for any differing declaration, range included, got warnings=%v", warningCodes(doc))
				Expect(declared).To(Equal("^5.4.0"))
				Expect(found).To(Equal("7.0.2"))

				Expect(text).To(ContainSubstring("compiler_declaration_mismatch (declared_version=^5.4.0 found_version=7.0.2 declaration_origin=manifest root=.)"))
			})
		})
	})
})

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: mise-tool-version and config-hazard gating (SA-280-015/SA-280-045, AC-9/AC-11)", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
		commitFile(repo, "project.json", singleRootPolicyJSON)
	})

	When("the mise tool's own version falls outside the frozen row", func() {
		BeforeEach(func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
		})

		It("rejects both mise scopes with package_manager_version_unsupported, never resolving the compiler through either", func() {
			path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", "2025.1.0 linux-x64 (2025-01-01)", "[]", "7.0.2")

			doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("fail"), "an out-of-row mise tool version must never let mise supply the compiler, got %+v", doc.Checks.Compiler)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(gapEntries(doc)).To(ContainElements(
				"package_manager_version_unsupported:mise_project",
				"package_manager_version_unsupported:mise_global",
			), "got gaps=%+v", doc.Gaps)
		})

		It("still passes when the project manifest origin independently resolves a supported compiler", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")

			path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", "2025.1.0 linux-x64 (2025-01-01)", "[]", "7.0.2")

			doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "mise's own rejection must never block a different, already-working origin, got %+v", doc.Checks.Compiler)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(gapEntries(doc)).NotTo(ContainElement(ContainSubstring("package_manager_version_unsupported")), "a passing compiler check must withhold every package_manager_* finding")
		})

		// isMiseToolVersionInRow's own unit tests already prove a newer calver
		// year is out of row (TestIsMiseToolVersionInRow); the sibling case
		// above only ever drives a full acceptance run through an older
		// (2025.x) year. Without this, nothing at this boundary proves the
		// newer direction reaches the same gap through the real
		// evaluateMiseToolVersionReadiness/aggregation pipeline rather than
		// only the isolated classifier.
		It("rejects both mise scopes with package_manager_version_unsupported for a newer-than-row version too, not only an older one", func() {
			path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", "2027.1.0 linux-x64 (2027-01-01)", "[]", "7.0.2")

			doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("fail"), "a newer-than-row mise tool version must never let mise supply the compiler, got %+v", doc.Checks.Compiler)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(gapEntries(doc)).To(ContainElements(
				"package_manager_version_unsupported:mise_project",
				"package_manager_version_unsupported:mise_global",
			), "got gaps=%+v", doc.Gaps)
		})
	})

	When("the mise tool's own version is rejected for two different reasons across two runs", func() {
		BeforeEach(func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
		})

		// The plan is explicit that every rejection reason's rendered
		// remediation must be textually distinct from the unverifiable
		// case's -- not merely a different JSON code coexisting with it.
		// This drives the same fixture through both stub-mise shapes and
		// diffs the actual rendered text reports, rather than only
		// asserting the two gap codes both appear somewhere in gaps[].
		It("renders textually distinct remediation for the unsupported and the unverifiable cases", func() {
			unsupportedPath, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", "2025.1.0 linux-x64 (2025-01-01)", "[]", "7.0.2")
			unsupportedDoc, unsupportedText := checkProjectBothFormats(repo, unsupportedPath, "--project-config", "project.json")
			Expect(unsupportedDoc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"), "sanity: fixture must actually reach the unsupported gap")

			unverifiablePath := pathWithStubNode("v24.9.9")
			unverifiableDoc, unverifiableText := checkProjectBothFormats(repo, unverifiablePath, "--project-config", "project.json")
			Expect(unverifiableDoc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"), "sanity: fixture must actually reach the unverifiable gap")

			Expect(unsupportedText).To(ContainSubstring("package_manager_version_unsupported package_manager_kind=mise_project"))
			Expect(unverifiableText).To(ContainSubstring("package_manager_version_unverifiable package_manager_kind=mise_project"))

			Expect(unsupportedText).NotTo(Equal(unverifiableText), "the two rejection reasons must never render identical remediation text")
			Expect(unsupportedText).NotTo(ContainSubstring("package_manager_version_unverifiable"), "the unsupported case's report must never also carry the unverifiable case's line")
			Expect(unverifiableText).NotTo(ContainSubstring("package_manager_version_unsupported"), "the unverifiable case's report must never also carry the unsupported case's line")
		})
	})

	When("the mise tool's own version cannot be determined at all", func() {
		BeforeEach(func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
		})

		It("rejects both mise scopes with package_manager_version_unverifiable, never resolving the compiler through either", func() {
			path := pathWithStubNode("v24.9.9")

			doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(gapEntries(doc)).To(ContainElements(
				"package_manager_version_unverifiable:mise_project",
				"package_manager_version_unverifiable:mise_global",
			), "got gaps=%+v", doc.Gaps)
		})

		It("still passes when the project manifest origin independently resolves a supported compiler", func() {
			writeInstalledTypescript(repo, "7.0.2")

			path := pathWithStubNode("v24.9.9")

			doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "mise being entirely undetectable must never block a different, already-working origin, got %+v", doc.Checks.Compiler)
			Expect(gapEntries(doc)).NotTo(ContainElement(ContainSubstring("package_manager_version_unverifiable")))
		})

		// Every other spec in this When block strips mise from PATH entirely
		// (pathWithStubNode), so the probe fails at exec.LookPath -- the same
		// fixture AC-17's separate "missing mise" item already covers, never
		// probeMiseToolVersion's own exitErr != nil / out == "" branches. This
		// spec puts a reachable stub mise on PATH whose `--version` itself
		// fails (writeVersionedStubMiseScript's toolVersionOutput == ""), so a
		// regression that skipped the `--version` gate whenever mise is
		// reachable -- and fell through to answering `config get`/`where`
		// instead -- would leave every mise-absent spec above green while this
		// one alone catches it.
		It("rejects both mise scopes with package_manager_version_unverifiable when mise is reachable but its own --version probe fails", func() {
			path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", "", "[]", "7.0.2")

			doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(gapEntries(doc)).To(ContainElements(
				"package_manager_version_unverifiable:mise_project",
				"package_manager_version_unverifiable:mise_global",
			), "got gaps=%+v", doc.Gaps)
		})
	})

	When("the project mise.toml carries an execution hazard", func() {
		hazardousMiseToml := "[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[hooks]\npostinstall = \"echo pwned\"\n"

		It("rejects only the project mise scope with package_manager_config_unverifiable, producing no side effect against that config", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "yarn.lock", "")
			commitFile(repo, "mise.toml", hazardousMiseToml)

			path, miseDir := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

			doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(gapEntries(doc)).To(ContainElements(
				"package_manager_config_unverifiable:mise_project",
				"package_manager_version_unsupported:yarn",
			), "got gaps=%+v", doc.Gaps)
			Expect(gapEntries(doc)).NotTo(ContainElement(ContainSubstring(":mise_global")), "the untrusted project scope must never withhold the still-trusted global scope")

			choices, ok := prepareCompilerChoices(doc)
			Expect(ok).To(BeTrue(), "a rejected project adapter must restrict prepare_compiler to the choices still verified")
			Expect(choices).To(Equal([]string{"mise_global"}), "the hazardous project scope and the rejected yarn adapter must both be withheld, leaving only mise_global")

			if invocations, err := os.ReadFile(filepath.Join(miseDir, stubMiseInvocationLog)); err == nil {
				Expect(string(invocations)).NotTo(ContainSubstring("where npm:typescript@7.0.2"), "a hazardous project mise.toml must never be located/installed from, got invocations=%s", invocations)
			}
		})

		It("still passes when the project manifest origin independently resolves a supported compiler", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "mise.toml", hazardousMiseToml)

			path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

			doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "a hazardous mise.toml must never block a different, already-working origin, got %+v", doc.Checks.Compiler)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(gapEntries(doc)).NotTo(ContainElement(ContainSubstring("package_manager_config_unverifiable")))
		})

		It("resolves normally from a clean mise.toml carrying only [tools], the hazard fixture's negative control", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, miseDir := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

			doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "a hazard-free project mise.toml must resolve exactly as before, got %+v", doc.Checks.Compiler)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(gapEntries(doc)).To(BeEmpty())

			invocations, err := os.ReadFile(filepath.Join(miseDir, stubMiseInvocationLog))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(invocations)).To(ContainSubstring("where npm:typescript@7.0.2"), "a hazard-free project mise.toml must still be located from, unlike its hazardous counterpart above")
		})

		// DescribeTable below covers TOML-legal hazard spellings that a naive
		// bracket-prefix scan would miss: whitespace inside the section
		// brackets, quoted section-header keys, and mise's bare top-level
		// inline-table shorthand for a [hooks]/[registry] table. Real mise
		// (verified against 2026.9.5) refuses each of these as untrusted
		// config, or -- for a global-scope config, which is exempt from
		// mise's trust gate -- actually executes the declared hook;
		// hasMiseConfigHazard must flag every one. The bare top-level
		// inline-table entry places `hooks = { ... }` before [tools] because
		// TOML parses a key after a table header as belonging to that table
		// (`tools.hooks`, not a top-level `hooks` table); mise treats the
		// version quoted here as a real hooks table only when it precedes
		// any header.
		DescribeTable("rejects TOML-legal hazard spellings a naive bracket-prefix scan would miss",
			func(hazardousToml string) {
				commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
				commitFile(repo, "yarn.lock", "")
				commitFile(repo, "mise.toml", hazardousToml)

				path, miseDir := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

				doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("fail"))
				Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
				Expect(gapEntries(doc)).To(ContainElement("package_manager_config_unverifiable:mise_project"), "got gaps=%+v", doc.Gaps)

				if invocations, err := os.ReadFile(filepath.Join(miseDir, stubMiseInvocationLog)); err == nil {
					Expect(string(invocations)).NotTo(ContainSubstring("where npm:typescript@7.0.2"), "a hazardous project mise.toml must never be located/installed from, got invocations=%s", invocations)
				}
			},
			Entry("whitespace inside single-bracket header: [ hooks ]",
				"[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[ hooks ]\npostinstall = \"echo pwned\"\n"),
			Entry("whitespace inside double-bracket header: [[ registry.mytool ]]",
				"[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[[ registry.mytool ]]\n"),
			Entry("double-quoted section header: [\"hooks\"]",
				"[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[\"hooks\"]\npostinstall = \"echo pwned\"\n"),
			Entry("single-quoted section header: ['hooks']",
				"[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n['hooks']\npostinstall = \"echo pwned\"\n"),
			Entry("bare top-level inline table: hooks = { enter = ... }",
				"hooks = { enter = \"echo pwned\" }\n\n[tools]\n\"npm:typescript\" = \"7.0.2\"\n"),
		)
	})
})

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: mise-tool resolution ignores a Bun declaration (AC-14, issue #354 out of scope)", func() {
	// AC-14 pins that the frozen row's install target names exactly one
	// version, never a secondary/fallback one; the unit-level guard
	// (TestMiseInstallCommandTakesExactlyOneVersionParameter) proves that
	// structurally, on miseInstallCommand's own signature. Nothing before
	// this drove a real mise.toml declaring a primary Bun version alongside
	// npm:typescript through the full readiness pipeline to prove that
	// declaration is never picked up as a runtime/compiler version by this
	// flow -- a config shape mise itself accepts today.
	It("resolves the compiler from the npm:typescript entry alone, never surfacing a Bun version declared alongside it in the same [tools] table", func() {
		repo := newTempGitRepo()
		commitFile(repo, "project.json", singleRootPolicyJSON)
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "mise.toml", "[tools]\nbun = \"1.2.3\"\n\"npm:typescript\" = \"7.0.2\"\n")

		path, miseDir := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

		doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

		Expect(doc.Checks.Compiler.State).To(Equal("pass"), "got %+v", doc.Checks.Compiler)
		Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"), "the resolved version must come from the npm:typescript entry, never the bun entry's own version")
		Expect(gapCodes(doc)).To(BeEmpty())
		Expect(text).NotTo(ContainSubstring("1.2.3"), "a mise-declared Bun version must never be consumed as a runtime declaration anywhere in this flow's rendered report, got:\n%s", text)

		// The stub's `where` branch echoes the same installDir regardless of
		// the version argument it was called with, so a passing
		// Checks.Compiler.Version/empty-gaps/no-"1.2.3" assertion alone
		// cannot distinguish "resolved from npm:typescript@7.0.2" from
		// "resolved from bun@1.2.3" -- both would satisfy every assertion
		// above. Only the recorded argv proves which tool identifier the
		// parser actually located/installed.
		invocations := readStubMiseInvocations(miseDir)
		Expect(invocations).To(ContainElement("where npm:typescript@7.0.2"), "the npm:typescript entry must be located from, got invocations=%v", invocations)
		for _, invocation := range invocations {
			Expect(invocation).NotTo(ContainSubstring("1.2.3"), "the bun entry's own version must never appear in any mise invocation, got invocations=%v", invocations)
		}
	})
})

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: mise.toml layout resolution is worktree-root-only (SA-280-043/044)", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
	})

	// compilerWorktreeRoot(dir) resolves `git rev-parse --show-toplevel` for
	// the analyzed directory and every mise-origin read
	// (readMiseProjectConfigFile) joins "mise.toml" onto that single root --
	// there is no per-selected-root or subdirectory walk. The two specs
	// below pin that actual, current behavior with a js/semantics-shaped
	// nested project (mirroring this repository's own layout, and the
	// existing project-manifest walk-up/walk-down specs above in
	// project_readiness_acceptance_test.go), applied to mise.toml
	// resolution specifically: a root-level mise.toml governs a nested
	// project root, and a mise.toml committed at the nested project root
	// itself is never discovered.
	When("the TypeScript project lives in a nested subdirectory and mise.toml lives at the worktree root", func() {
		It("still resolves the compiler through the root-level mise.toml, regardless of where the project itself lives", func() {
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["js/semantics"]}`+"\n")
			commitFile(repo, "js/semantics/package.json", `{"name":"semantics","version":"1.0.0"}`+"\n")
			commitFile(repo, "js/semantics/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "no top-level package.json: the pass below must come from mise, not the already-covered project-manifest origin")

			path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

			doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "a root-level mise.toml must resolve the compiler for a nested project root too, got %+v", doc.Checks.Compiler)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(gapCodes(doc)).To(BeEmpty())
			Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
		})
	})

	When("mise.toml instead lives inside the nested project subdirectory, never at the worktree root", func() {
		It("never resolves the compiler through it: today's mise-origin resolution only ever reads the worktree root", func() {
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["js/semantics"]}`+"\n")
			commitFile(repo, "js/semantics/package.json", `{"name":"semantics","version":"1.0.0"}`+"\n")
			commitFile(repo, "js/semantics/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "js/semantics/mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			_, statErr := os.Stat(filepath.Join(repo, "mise.toml"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "sanity: no worktree-root mise.toml must exist in this negative control")

			path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

			doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("fail"), "a mise.toml outside the worktree root must never be discovered, got %+v", doc.Checks.Compiler)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(text).To(ContainSubstring("origins=project:unconfigured,mise_project:unconfigured,mise_global:unconfigured"), "mise_project must report unconfigured -- the worktree-root file was never even present to read, got %s", text)
		})
	})
})
