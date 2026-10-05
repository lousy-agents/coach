package main

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func body_projectTsScanPreflightAcceptanceTest_270(major int) {
	version := fmt.Sprintf("v%d.0.0", major)
	path := pathWithStubNode(version)
	GinkgoT().Setenv("PATH", path)
	GinkgoT().Setenv("HOME", os.Getenv("HOME"))

	repo := newTempGitRepo()
	head := commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

	readiness, err := codesignalcli.CheckProjectReadiness(repo, head, "")
	Expect(err).NotTo(HaveOccurred())

	Expect(readiness.Checks.Node.State).To(Equal(projectreadiness.Pass), "detail=%s", readiness.Checks.Node.Detail)
	Expect(readiness.Checks.Node.Code).To(BeEmpty())
	Expect(readiness.Checks.Runtime.State).To(Equal(projectreadiness.Pass), "detail=%s", readiness.Checks.Runtime.Detail)
	Expect(readiness.Checks.Runtime.Code).To(BeEmpty())

	for _, gap := range readiness.Gaps {
		Expect(gap.Code).NotTo(HavePrefix("node_"), "a supported-set Node release must never contribute a node_* gap, got %q", gap.Code)
	}
	for _, warning := range readiness.Warnings {
		Expect(warning.Code).NotTo(HavePrefix("node_"), "a supported-set Node release must never warn, got %q", warning.Code)
	}
	for _, action := range readiness.NextActions {
		Expect(action.RuntimeKind).NotTo(Equal("node"), "a supported-set Node release must never contribute a runtime next action, got kind=%q", action.Kind)
	}

	_, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
	Expect(exitCode).To(Equal(2), "stdout/stderr: %s", stderr)
	Expect(stderrLines(stderr)[0]).To(HavePrefix("typescript_compiler_missing:"), "the scan must fail on the later missing-compiler boundary, not on Node; stderr: %s", stderr)
}
