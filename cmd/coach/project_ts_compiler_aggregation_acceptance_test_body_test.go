package main

import (
	"encoding/json"

	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func body_projectTsCompilerAggregationAcceptanceTest_passesFromThatOriginRatherThanReportingACompiler_194(repo string) {
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
}

func body_projectTsCompilerAggregationAcceptanceTest_warnsOncePerDisagreeingRootInThePolicySRootsOrde_349(repo string) {
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
}

func body_projectTsCompilerAggregationAcceptanceTest_checksCompilerCarriesOnlyTheFrozenFieldsWhatever_380(fixture func(repo string), miseVersion string, wantCode string, repo string) {
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
}

func body_projectTsCompilerAggregationAcceptanceTest_usesAPrivatePerInvocationWorkingDirectoryNotAFix_491() {
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
}

func body_projectTsCompilerAggregationAcceptanceTest_rejectsOnlyTheProjectMiseScopeWithPackageManager_764(repo string, hazardousMiseToml string) {
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	commitFile(repo, "yarn.lock", "")
	commitFile(repo, "mise.toml", hazardousMiseToml)

	path, miseDir := pathWithVersionedStubMise("v24.9.9", "", defaultStubMiseToolVersion, "[]", "7.0.2")

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
		Expect(string(invocations)).NotTo(ContainSubstring("install "), "a hazardous project mise.toml must never be installed from, got invocations=%s", invocations)
	}
}

func body_projectTsCompilerAggregationAcceptanceTest_rejectsTOMLLegalHazardSpellingsANaiveBracketPref_822(hazardousToml string, repo string) {
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
}

func body_projectTsCompilerAggregationAcceptanceTest_resolvesTheCompilerFromTheNpmTypescriptEntryAlon_864() {
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

	invocations := readStubMiseInvocations(miseDir)
	Expect(invocations).To(ContainElement("where npm:typescript@7.0.2"), "the npm:typescript entry must be located from, got invocations=%v", invocations)
	for _, invocation := range invocations {
		Expect(invocation).NotTo(ContainSubstring("1.2.3"), "the bun entry's own version must never appear in any mise invocation, got invocations=%v", invocations)
	}
}
