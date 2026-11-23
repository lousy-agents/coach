package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal scope.policy.matches_head on a converted checkout (#168 item 5)", func() {
	const (
		customPolicyPath = "policies/custom.json"
		// reordered carries the same policy as singleRootPolicyJSON in different bytes, so it is an edit.
		reordered = `{"roots":["."],"schema_version":"1"}` + "\n"
	)
	crlf := func(body string) string { return strings.ReplaceAll(body, "\n", "\r\n") }
	digestOfCommitted := func(body string) string {
		sum := sha256.Sum256([]byte(body))
		return "pcfg_" + hex.EncodeToString(sum[:])
	}
	autocrlf := func(repo string) { runGitInRepo(repo, "config", "core.autocrlf", "true") }
	// checkedOutByGit is how a Windows checkout comes to hold converted bytes: git writes the file and records the
	// size on disk, so git status needs no stat-dirty comparison to call it clean.
	checkedOutByGit := func(repo, path string) {
		Expect(os.Remove(filepath.Join(repo, path))).To(Succeed())
		runGitInRepo(repo, "checkout", "--", path)
	}

	type convertedCheckout struct {
		description string
		policies    []scopePolicyFile
		arrange     func(repo string)
		extraArgs   []string
		policyPath  string
		// cleanInGit is whether git itself calls the policy file unmodified after arrange, which is what makes a
		// matches_head of true the right answer rather than a loose one.
		cleanInGit   bool
		wantMatching bool
	}
	scenarios := []convertedCheckout{
		{
			description: "AC-C1 a policy committed with LF and checked out with CRLF by core.autocrlf",
			policies:    []scopePolicyFile{{coachConfigPath, singleRootPolicyJSON}},
			arrange: func(repo string) {
				autocrlf(repo)
				checkedOutByGit(repo, coachConfigPath)
			},
			policyPath: coachConfigPath, cleanInGit: true, wantMatching: true,
		},
		{
			description: "AC-C2 a policy committed with LF and checked out with CRLF by a committed `text eol=crlf` attribute",
			policies:    []scopePolicyFile{{".gitattributes", "* text eol=crlf\n"}, {coachConfigPath, singleRootPolicyJSON}},
			arrange:     func(repo string) { checkedOutByGit(repo, coachConfigPath) },
			policyPath:  coachConfigPath, cleanInGit: true, wantMatching: true,
		},
		{
			description: "AC-C3 a policy named with --project-config and checked out with CRLF by core.autocrlf",
			policies:    []scopePolicyFile{{customPolicyPath, singleRootPolicyJSON}},
			arrange: func(repo string) {
				autocrlf(repo)
				checkedOutByGit(repo, customPolicyPath)
			},
			extraArgs:  []string{"--project-config", customPolicyPath},
			policyPath: customPolicyPath, cleanInGit: true, wantMatching: true,
		},
		{
			description: "AC-C4 an edited policy that also has CRLF line endings under core.autocrlf (guard: the edit alone flips the outcome)",
			policies:    []scopePolicyFile{{coachConfigPath, singleRootPolicyJSON}},
			arrange: func(repo string) {
				autocrlf(repo)
				scopePolicyRewrite(repo, coachConfigPath, crlf(reordered))
			},
			policyPath: coachConfigPath, wantMatching: false,
		},
		{
			description: "AC-C5 a CRLF policy under core.autocrlf whose path selects a filter (guard: the filter alone flips the outcome, and no program is run)",
			policies:    []scopePolicyFile{{".gitattributes", coachConfigPath + " filter=unusedfilter\n"}, {coachConfigPath, singleRootPolicyJSON}},
			arrange: func(repo string) {
				runGitInRepo(repo, "config", "filter.unusedfilter.clean", "cat")
				autocrlf(repo)
				checkedOutByGit(repo, coachConfigPath)
			},
			policyPath: coachConfigPath, wantMatching: false,
		},
	}

	for _, mode := range scopePolicyModes {
		for _, scenario := range scenarios {
			It("#168 item 5: converted checkout: scope.policy.matches_head is "+boolWord(scenario.wantMatching)+" for "+scenario.description+", under "+mode.name+", with the matching policy line (and WARNING for an edit) in the text", func() {
				repo, base := scopePolicyRepo(scenario.policies...)
				scenario.arrange(repo)
				onDisk, err := os.ReadFile(filepath.Join(repo, scenario.policyPath))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(onDisk)).To(ContainSubstring("\r\n"), "the fixture's bytes on disk must be CRLF")
				if scenario.cleanInGit {
					Expect(runGitInRepo(repo, "status", "--porcelain", "--", scenario.policyPath)).To(BeEmpty(),
						"the fixture must be a converted checkout that git calls clean, or matches_head true proves nothing")
				}
				jsonArgs := append(append([]string{}, scenario.extraArgs...), "--format=json")
				// A converted checkout is the committed policy, so its digest is the committed blob's; any other file is read as it is on disk.
				usedBody := string(onDisk)
				wantSuffix, wantWarning, wantCause := "differs from HEAD.", diskPolicyDiffersWarning, diskCauseDiffersFromHead
				if scenario.wantMatching {
					usedBody, wantSuffix, wantWarning, wantCause = singleRootPolicyJSON, "same as HEAD.", "", diskCauseSameAsHead
				}

				jsonOut, stderr, exitCode := mode.run(repo, base, jsonArgs...)
				Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
				Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(map[string]any{
					"path":         scenario.policyPath,
					"digest":       digestOfCommitted(usedBody),
					"matches_head": scenario.wantMatching,
					"cause":        wantCause,
				})))

				text, stderr, exitCode := mode.run(repo, base, scenario.extraArgs...)
				Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
				expectDiskPolicyDisclosure(mode, text, diskPolicyLine(scenario.policyPath, usedBody, wantSuffix), wantWarning)
			})
		}
	}
})
