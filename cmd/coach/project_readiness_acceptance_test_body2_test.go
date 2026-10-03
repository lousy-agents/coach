package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/gomega"
)

func body_projectReadinessAcceptanceTest_reportsNodeFailNodeUnverifiableWithABoundedPathF_818() {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
	writeInstalledTypescript(repo, "7.0.2")
	commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

	path := pathWithFailingNode()

	stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
	Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

	var doc readinessResultDoc
	Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
	Expect(doc.SchemaVersion).NotTo(BeEmpty(), "the readiness document must still be produced on a probe failure")
	Expect(doc.Checks.Node.State).To(Equal("fail"))
	Expect(doc.Checks.Node.Code).To(Equal("node_unverifiable"))
	Expect(doc.Checks.Node.Detail).To(BeEmpty(), "checks.node must never mirror detail")
	Expect(doc.Checks.Runtime.State).To(Equal("fail"))
	Expect(doc.Checks.Runtime.Code).To(Equal("node_unverifiable"))
	Expect(doc.Checks.Runtime.Kind).To(Equal("node"))
	Expect(doc.Checks.Runtime.Origin).To(Equal("path"))
	Expect(doc.Checks.Runtime.Detail).To(Equal("node --version failed: running node --version: exit status 3"))
	Expect(doc.Checks.Runtime.Detail).NotTo(ContainSubstring(string(os.PathSeparator)), "the detail must not leak the stub node's host path")
	Expect(doc.Status).To(Equal("needs_prerequisite"))

	var action readinessNextActionDoc
	for _, a := range doc.NextActions {
		if a.Kind == "repair_runtime_probe" {
			action = a
		}
	}
	Expect(action.Kind).To(Equal("repair_runtime_probe"))
	Expect(action.Executable).To(BeFalse())
	Expect(action.RuntimeKind).To(Equal("node"))
	Expect(action.Supported).To(BeEmpty(), "repair_runtime_probe carries no supported field")
	Expect(action.FoundVersion).To(BeEmpty(), "repair_runtime_probe carries no found_version field")
	Expect(action.Detail).To(Equal("node --version failed: running node --version: exit status 3"))

	textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
	Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
	text := string(textStdout)
	Expect(text).To(ContainSubstring("runtime: fail (node_unverifiable) kind=node origin=path"))
	Expect(text).To(ContainSubstring("repair_runtime_probe (executable=false) runtime_kind=node detail=node --version failed: running node --version: exit status 3"))
}

func body_projectReadinessAcceptanceTest_reportsNodeFailNodeUnverifiableWithABoundedPathF_865() {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
	writeInstalledTypescript(repo, "7.0.2")
	commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

	path := pathWithUnstartableNode()

	stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
	Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

	var doc readinessResultDoc
	Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
	Expect(doc.SchemaVersion).NotTo(BeEmpty(), "the readiness document must still be produced on a probe failure")
	Expect(doc.Checks.Node.State).To(Equal("fail"))
	Expect(doc.Checks.Node.Code).To(Equal("node_unverifiable"))
	Expect(doc.Checks.Node.Detail).To(BeEmpty(), "checks.node must never mirror detail")
	Expect(doc.Checks.Runtime.State).To(Equal("fail"))
	Expect(doc.Checks.Runtime.Code).To(Equal("node_unverifiable"))
	Expect(doc.Checks.Runtime.Kind).To(Equal("node"))
	Expect(doc.Checks.Runtime.Origin).To(Equal("path"))
	Expect(doc.Checks.Runtime.Detail).To(Equal("node --version failed to start: no such file or directory"))
	Expect(doc.Checks.Runtime.Detail).NotTo(ContainSubstring(string(os.PathSeparator)), "the detail must not leak the stub node's host path")
	Expect(doc.Checks.Runtime.Detail).NotTo(ContainSubstring("fork/exec"), "the detail must not leak the raw fork/exec diagnostic")
	Expect(doc.Status).To(Equal("needs_prerequisite"))

	var action readinessNextActionDoc
	for _, a := range doc.NextActions {
		if a.Kind == "repair_runtime_probe" {
			action = a
		}
	}
	Expect(action.Kind).To(Equal("repair_runtime_probe"))
	Expect(action.Executable).To(BeFalse())
	Expect(action.RuntimeKind).To(Equal("node"))
	Expect(action.Supported).To(BeEmpty(), "repair_runtime_probe carries no supported field")
	Expect(action.FoundVersion).To(BeEmpty(), "repair_runtime_probe carries no found_version field")
	Expect(action.Detail).To(Equal("node --version failed to start: no such file or directory"))

	textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
	Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
	text := string(textStdout)
	Expect(text).To(ContainSubstring("runtime: fail (node_unverifiable) kind=node origin=path"))
	Expect(text).To(ContainSubstring("repair_runtime_probe (executable=false) runtime_kind=node detail=node --version failed to start: no such file or directory"))
}

func body_projectReadinessAcceptanceTest_reportsTheCompilerCheckAsFailTypescriptVersionMi_1197() {
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

	var action readinessNextActionDoc
	for _, a := range doc.NextActions {
		if a.Kind == "prepare_compiler" {
			action = a
		}
	}
	Expect(action.Kind).To(Equal("prepare_compiler"))
	Expect(action.Executable).To(BeTrue())
	Expect(action.RuntimeKind).To(BeEmpty(), "prepare_compiler carries no runtime_kind field")
	Expect(action.Supported).To(Equal([]string{"7.0.2"}))
	Expect(action.FoundVersion).To(Equal("5.4.0"))
	Expect(action.Detail).To(BeEmpty(), "prepare_compiler carries no detail field")
}

func body_projectReadinessAcceptanceTest_passesFromTheProjectOriginWhichOutranksTheMisePi_1484() {
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
}

func body_projectReadinessAcceptanceTest_producesNoSideEffectDuringCheckProjectBecauseMis_1543() {
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
}
