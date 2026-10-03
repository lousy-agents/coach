package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func body_projectReadinessAcceptanceTest_reportsStatusNeedsPrerequisiteWithPolicyMissingA_391() {
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
}

func body_projectReadinessAcceptanceTest_reportsPolicyFailPolicyInvalidAndExits0QuicklyRa_486() {
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
}

func body_projectReadinessAcceptanceTest_reportsTheNodeCheckAsFailNodeMissingDeterministi_523() {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
	writeInstalledTypescript(repo, "7.0.2")
	commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

	path := pathWithoutNode()
	requireNodeUnreachable(path)

	stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
	Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

	var doc readinessResultDoc
	Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
	Expect(doc.Checks.Node.State).To(Equal("fail"))
	Expect(doc.Checks.Node.Code).To(Equal("node_missing"))
	Expect(doc.Checks.Node.Kind).To(BeEmpty(), "checks.node must never mirror kind")
	Expect(doc.Checks.Node.Origin).To(BeEmpty(), "checks.node must never mirror origin")
	Expect(doc.Checks.Runtime.State).To(Equal("fail"))
	Expect(doc.Checks.Runtime.Code).To(Equal("node_missing"))
	Expect(doc.Checks.Runtime.Kind).To(Equal("node"))
	Expect(doc.Checks.Runtime.Origin).To(Equal("path"))
	Expect(doc.Checks.Node.State).To(Equal(doc.Checks.Runtime.State))
	Expect(doc.Checks.Node.Code).To(Equal(doc.Checks.Runtime.Code))
	Expect(doc.Checks.Node.Version).To(Equal(doc.Checks.Runtime.Version))
	Expect(doc.Status).To(Equal("needs_prerequisite"))

	var action readinessNextActionDoc
	for _, a := range doc.NextActions {
		if a.Kind == "install_supported_runtime" {
			action = a
		}
	}
	Expect(action.Kind).To(Equal("install_supported_runtime"))
	Expect(action.Executable).To(BeFalse())
	Expect(action.RuntimeKind).To(Equal("node"))
	Expect(action.Supported).To(Equal([]string{"24", "26"}))
	Expect(action.FoundVersion).To(BeEmpty(), "node_missing has no probed version to report")
	Expect(action.Detail).To(BeEmpty(), "install_supported_runtime carries no detail field")

	textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
	Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
	text := string(textStdout)
	Expect(text).To(ContainSubstring("runtime: fail (node_missing) kind=node origin=path"))
	Expect(text).To(ContainSubstring("install_supported_runtime (executable=false) runtime_kind=node supported=24,26"))
}

func body_projectReadinessAcceptanceTest_reportsTheNodeCheckAsFailNodeUnsupportedWithTheF_572() {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
	writeInstalledTypescript(repo, "7.0.2")
	commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

	path := pathWithStubNode("v22.10.0")
	requireStubNodeVersion(path, "v22.10.0")

	stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
	Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

	var doc readinessResultDoc
	Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
	Expect(doc.Checks.Node.State).To(Equal("fail"))
	Expect(doc.Checks.Node.Code).To(Equal("node_unsupported"))
	Expect(doc.Checks.Node.Version).To(Equal("v22.10.0"))
	Expect(doc.Checks.Runtime.State).To(Equal("fail"))
	Expect(doc.Checks.Runtime.Code).To(Equal("node_unsupported"))
	Expect(doc.Checks.Runtime.Version).To(Equal("v22.10.0"))
	Expect(doc.Checks.Runtime.Kind).To(Equal("node"))
	Expect(doc.Checks.Runtime.Origin).To(Equal("path"))
	Expect(doc.Status).To(Equal("needs_prerequisite"))

	var action readinessNextActionDoc
	for _, a := range doc.NextActions {
		if a.Kind == "install_supported_runtime" {
			action = a
		}
	}
	Expect(action.Kind).To(Equal("install_supported_runtime"))
	Expect(action.Executable).To(BeFalse())
	Expect(action.RuntimeKind).To(Equal("node"))
	Expect(action.Supported).To(Equal([]string{"24", "26"}))
	Expect(action.FoundVersion).To(Equal("v22.10.0"), "node_unsupported carries the probed version that fell outside the supported set")
	Expect(action.Detail).To(BeEmpty(), "install_supported_runtime carries no detail field")
}

func body_projectReadinessAcceptanceTest_stillExitsWithinABoundedWallClockReportingNodeFa_771() {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
	writeInstalledTypescript(repo, "7.0.2")
	commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

	path := pathWithHangingNode()

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
	Expect(doc.Checks.Runtime.Detail).To(Equal("node --version timed out"))
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
	Expect(action.Detail).To(Equal("node --version timed out"))

	textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
	Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
	text := string(textStdout)
	Expect(text).To(ContainSubstring("runtime: fail (node_unverifiable) kind=node origin=path"))
	Expect(text).To(ContainSubstring("repair_runtime_probe (executable=false) runtime_kind=node detail=node --version timed out"))
}
