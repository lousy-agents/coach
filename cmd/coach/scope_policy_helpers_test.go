package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

// scopePolicyMode is one way to ask for a scan: against the baseline or against a --base, each with and
// without --worktree. golden names the v0.12.0 text file the mode's output is pinned to.
type scopePolicyMode struct {
	name   string
	diff   bool
	args   []string
	golden string
}

var scopePolicyModes = []scopePolicyMode{
	{name: "--baseline", diff: false, golden: "baseline"},
	{name: "--baseline --worktree", diff: false, args: []string{"--worktree"}, golden: "baseline_worktree"},
	{name: "--base", diff: true, golden: "base"},
	{name: "--base --worktree", diff: true, args: []string{"--worktree"}, golden: "base_worktree"},
}

func (m scopePolicyMode) run(repo, base string, extra ...string) (stdout, stderr []byte, exitCode int) {
	args := append(append([]string{}, m.args...), extra...)
	if m.diff {
		return runCoachCodesignalRaw(repo, base, args...)
	}
	return runCoachCodesignalBaselineRaw(repo, args...)
}

type scopePolicyFile struct {
	path string
	body string
}

// scopePolicyRepo commits a.go, then each policy file in order, then b.go. base is the commit before b.go, so a
// --base scan analyzes b.go and sees every policy file as part of the base.
func scopePolicyRepo(policies ...scopePolicyFile) (repo, base string) {
	repo = newTempGitRepo()
	base = commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
	for _, policy := range policies {
		base = commitFile(repo, policy.path, policy.body)
	}
	commitFile(repo, "b.go", "package a\n\nfunc B() {}\n")
	return repo, base
}

// scopePolicyNormalized replaces the commit identifiers of a scan's text with the placeholders the golden files use.
func scopePolicyNormalized(text, head, base string) string {
	replacer := strings.NewReplacer(head, "{head}", head[:12], "{head12}", base[:12], "{base12}")
	return replacer.Replace(text)
}

func scopePolicyHead(repo string) string {
	return strings.TrimSpace(runGitInRepo(repo, "rev-parse", "HEAD"))
}

func scopePolicyScopeOf(stdout []byte) map[string]any {
	var report struct {
		Scope map[string]any `json:"scope"`
	}
	ExpectWithOffset(1, json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)
	return report.Scope
}

func scopePolicyRewrite(repo, name, contents string) {
	ExpectWithOffset(1, os.WriteFile(filepath.Join(repo, name), []byte(contents), 0o644)).To(Succeed())
}
