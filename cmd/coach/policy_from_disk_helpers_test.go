package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const diskPolicyWithoutForbiddenImports = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}]}`

const diskPolicyDiffersWarning = "WARNING: the project policy on disk differs from HEAD's; this scan used the policy on disk, so its result can differ from CI, which reads the committed policy."

const diskPolicyQualifiedAllClear = "No active CodeSignal findings, but the project policy on disk differs from HEAD."

const diskPolicyLayerViolationRule = "architecture.layer_violation"

var diskPolicySummaryLines = map[string]*regexp.Regexp{
	"baseline":          regexp.MustCompile(`^tracked files discovered: `),
	"baseline_worktree": regexp.MustCompile(`^files discovered \(tracked and new\): `),
	"base":              regexp.MustCompile(`^scope: .*files analyzed: `),
	"base_worktree":     regexp.MustCompile(`^scope: .*files analyzed: `),
}

// diskPolicyRepo commits a Go project whose handlers import db, then each policy file in order, then b.go. base is
// the commit before b.go, so a --base scan sees every policy file as part of the base.
func diskPolicyRepo(policies ...scopePolicyFile) (repo, base string) {
	repo = newTempGitRepo()
	commitFile(repo, "go.mod", goModuleFile)
	commitFile(repo, "pkg/db/db.go", dbPackageFile)
	base = commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
	for _, policy := range policies {
		base = commitFile(repo, policy.path, policy.body)
	}
	commitFile(repo, "b.go", "package a\n\nfunc B() {}\n")
	return repo, base
}

func diskPolicyDigest(body string) string {
	sum := sha256.Sum256([]byte(body))
	return "pcfg_" + hex.EncodeToString(sum[:])
}

func diskPolicyScope(path, body string, matchesHead bool) map[string]any {
	return map[string]any{"path": path, "digest": diskPolicyDigest(body), "matches_head": matchesHead}
}

func diskPolicyLine(path, body, suffix string) string {
	return fmt.Sprintf("policy: %s (%s), %s", path, diskPolicyDigest(body)[:len("pcfg_")+12], suffix)
}

func diskPolicyNotCommittedWarning(path string) string {
	return fmt.Sprintf("WARNING: the project policy %s is on disk and not committed at HEAD; this scan used it.", path)
}

func diskPolicyMissingOnDiskWarning(path string) string {
	return fmt.Sprintf("WARNING: %s is committed at HEAD and missing on disk; this scan used the committed copy.", path)
}

// diskPolicyLinesAfterSummary returns up to count lines directly after the scan's summary line.
func diskPolicyLinesAfterSummary(mode scopePolicyMode, text []byte, count int) []string {
	lines := strings.Split(string(text), "\n")
	for i, line := range lines {
		if diskPolicySummaryLines[mode.golden].MatchString(line) {
			return lines[i+1 : min(i+1+count, len(lines))]
		}
	}
	ExpectWithOffset(1, string(text)).To(MatchRegexp("(?m)"+diskPolicySummaryLines[mode.golden].String()), "%s needs its summary line", mode.name)
	return nil
}

// expectDiskPolicyDisclosure asserts the policy line directly after the summary line and, when warning is empty,
// that the text carries no WARNING; otherwise the warning is the line after the policy line.
func expectDiskPolicyDisclosure(mode scopePolicyMode, text []byte, policyLine, warning string) {
	after := diskPolicyLinesAfterSummary(mode, text, 2)
	ExpectWithOffset(1, after).NotTo(BeEmpty(), "text:\n%s", text)
	ExpectWithOffset(1, after[0]).To(Equal(policyLine), "text:\n%s", text)
	if warning == "" {
		ExpectWithOffset(1, string(text)).NotTo(ContainSubstring("WARNING"))
		return
	}
	ExpectWithOffset(1, after).To(HaveLen(2), "text:\n%s", text)
	ExpectWithOffset(1, after[1]).To(Equal(warning), "text:\n%s", text)
}

func diskPolicyLayerViolations(report *codesignal.Report) []codesignal.ProjectChange {
	var found []codesignal.ProjectChange
	for _, change := range report.ProjectChanges {
		if change.RuleID == diskPolicyLayerViolationRule {
			found = append(found, change)
		}
	}
	return found
}

// diskPolicyJSONScan runs one scan to JSON and requires exit 0.
func diskPolicyJSONScan(mode scopePolicyMode, repo, base string, extra ...string) (*codesignal.Report, []byte) {
	jsonOut, stderr, exitCode := mode.run(repo, base, append(append([]string{}, extra...), "--format=json")...)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
	return decodeCoachReport(jsonOut), jsonOut
}

func diskPolicyTextScan(mode scopePolicyMode, repo, base string, extra ...string) []byte {
	text, stderr, exitCode := mode.run(repo, base, extra...)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
	return text
}

// expectGitStatusContains pins a fixture's on-disk state in git's own terms, so a spec cannot pass or fail on a
// fixture that never reached the state it names.
func expectGitStatusContains(repo, snippet string) {
	command := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	command.Dir = repo
	output, err := command.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git status: %s", output)
	ExpectWithOffset(1, string(output)).To(ContainSubstring(snippet))
}
