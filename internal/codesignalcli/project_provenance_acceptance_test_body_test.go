package codesignalcli

import (
	"context"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_projectProvenanceAcceptanceTest_appendsAWorktreeReportReflectsCommittedHeadDiagn_16() {
	original := listWorktreeStatus
	listWorktreeStatus = func(string) ([]worktreeStatusEntry, error) {
		return parseWorktreeStatus([]byte("M  bun.lock\x00")), nil
	}
	DeferCleanup(func() { listWorktreeStatus = original })

	dir := gitfixture.Init(GinkgoT())
	gitfixture.CommitFile(GinkgoT(), dir, "a.go", "package a\n")

	backend := identityHandoffBackend{result: &ProjectBackendResult{
		RuntimeKind:   runtimeKindNode,
		RuntimeOrigin: runtimeOriginPath,
	}}
	cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
	project := &ProjectAnalysis{
		ConfigPath:   "project.json",
		Language:     "typescript",
		Config:       cfg,
		ConfigDigest: ConfigDigest(cfg),
		Backend:      backend,
	}

	input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, project, dir, "HEAD", "", true)
	Expect(err).NotTo(HaveOccurred())

	kinds := make([]string, len(input.Diagnostics))
	for i, d := range input.Diagnostics {
		kinds[i] = d.Kind
	}
	Expect(kinds).To(ContainElement(codesignal.DiagKindWorktreeReportReflectsCommittedHEAD),
		"a dirty relevant path must trigger the worktree_report_reflects_committed_head diagnostic; diagnostics=%v", input.Diagnostics)
	Expect(kinds).NotTo(ContainElement(codesignal.DiagKindWorktreeChangesNotAnalyzed),
		"project provenance must not reuse the file-local skip kind; diagnostics=%v", input.Diagnostics)
}

func body_projectProvenanceAcceptanceTest_worktreeReportReflectsCommittedHeadMessageRefere_52() {
	original := listWorktreeStatus
	listWorktreeStatus = func(string) ([]worktreeStatusEntry, error) {
		return parseWorktreeStatus([]byte("?? bun.lock\x00")), nil
	}
	DeferCleanup(func() { listWorktreeStatus = original })

	dir := gitfixture.Init(GinkgoT())
	gitfixture.CommitFile(GinkgoT(), dir, "a.ts", "export const x = 1;\n")

	backend := identityHandoffBackend{result: &ProjectBackendResult{}}
	cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
	project := &ProjectAnalysis{
		ConfigPath:   "project.json",
		Language:     "typescript",
		Config:       cfg,
		ConfigDigest: ConfigDigest(cfg),
		Backend:      backend,
	}

	input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, project, dir, "HEAD", "", true)
	Expect(err).NotTo(HaveOccurred())

	var msg string
	for _, d := range input.Diagnostics {
		if d.Kind == codesignal.DiagKindWorktreeReportReflectsCommittedHEAD {
			msg = d.Message
		}
	}
	Expect(msg).NotTo(BeEmpty(), "diagnostic message must be set")
	Expect(msg).To(ContainSubstring("HEAD"), "message must reference committed HEAD; got %q", msg)
}

func body_projectProvenanceAcceptanceTest_doesNotEmitAWorktreeProvenanceDiagnosticWhenTheW_85() {
	original := listWorktreeStatus
	listWorktreeStatus = func(string) ([]worktreeStatusEntry, error) {
		return parseWorktreeStatus([]byte{}), nil
	}
	DeferCleanup(func() { listWorktreeStatus = original })

	dir := gitfixture.Init(GinkgoT())
	gitfixture.CommitFile(GinkgoT(), dir, "a.ts", "export const x = 1;\n")

	backend := identityHandoffBackend{result: &ProjectBackendResult{}}
	cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
	project := &ProjectAnalysis{
		ConfigPath:   "project.json",
		Language:     "typescript",
		Config:       cfg,
		ConfigDigest: ConfigDigest(cfg),
		Backend:      backend,
	}

	input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, project, dir, "HEAD", "", true)
	Expect(err).NotTo(HaveOccurred())

	for _, d := range input.Diagnostics {
		Expect(d.Kind).NotTo(Equal(codesignal.DiagKindWorktreeChangesNotAnalyzed),
			"clean worktree must not trigger the skip diagnostic")
		Expect(d.Kind).NotTo(Equal(codesignal.DiagKindWorktreeReportReflectsCommittedHEAD),
			"clean worktree must not trigger the provenance diagnostic")
	}
}

func body_projectProvenanceAcceptanceTest_emitsWorktreeStatusCheckFailedRatherThanDescribi_154() {
	original := listWorktreeStatus
	listWorktreeStatus = func(string) ([]worktreeStatusEntry, error) {
		return nil, errors.New("simulated git status failure")
	}
	DeferCleanup(func() { listWorktreeStatus = original })

	dir := gitfixture.Init(GinkgoT())
	gitfixture.CommitFile(GinkgoT(), dir, "a.ts", "export const x = 1;\n")

	backend := identityHandoffBackend{result: &ProjectBackendResult{}}
	cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
	project := &ProjectAnalysis{
		ConfigPath:   "project.json",
		Language:     "typescript",
		Config:       cfg,
		ConfigDigest: ConfigDigest(cfg),
		Backend:      backend,
	}

	input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, project, dir, "HEAD", "", true)
	Expect(err).NotTo(HaveOccurred())

	kinds := make([]string, len(input.Diagnostics))
	for i, d := range input.Diagnostics {
		kinds[i] = d.Kind
	}
	Expect(kinds).To(ContainElement(codesignal.DiagKindWorktreeStatusCheckFailed),
		"a git status error must emit worktree_status_check_failed; diagnostics=%v", input.Diagnostics)
	Expect(kinds).NotTo(ContainElement(codesignal.DiagKindWorktreeChangesNotAnalyzed),
		"a git status error must not be described as skipped analysis; diagnostics=%v", input.Diagnostics)
}

func body_projectProvenanceAcceptanceTest_234(dir string, args []string, originalRunner func(dir string, args ...string) ([]byte, error)) ([]byte, error) {
	if len(args) > 0 && args[0] == "show" {
		return nil, errors.New("simulated blob read failure")
	}
	return originalRunner(dir, args...)
}

func body_projectProvenanceAcceptanceTest_257(dir string, args []string, originalRunner func(dir string, args ...string) ([]byte, error)) ([]byte, error) {
	// Fail ls-tree calls so fileExistsAtRevision errors on lockfile checks.
	if len(args) > 0 && args[0] == "ls-tree" {
		return nil, errors.New("simulated transient git failure")
	}
	return originalRunner(dir, args...)
}
