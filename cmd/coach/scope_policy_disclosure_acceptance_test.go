package main

import (
	"crypto/sha256"
	"encoding/hex"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal discloses the policy a scan used in scope.policy (#168 A1)", func() {
	const (
		// reordered carries the same policy as singleRootPolicyJSON in different bytes, so its digest differs.
		reordered = `{"roots":["."],"schema_version":"1"}` + "\n"
		wantNull  = "the null shape: scope.policy is present and null"
	)
	digestOf := func(body string) string {
		sum := sha256.Sum256([]byte(body))
		return "pcfg_" + hex.EncodeToString(sum[:])
	}

	for _, mode := range scopePolicyModes {
		It("#168 item 9: with no policy anywhere, "+mode.name+" has no policy line in text and "+wantNull+" in JSON", func() {
			repo, base := scopePolicyRepo()

			jsonOut, stderr, exitCode := mode.run(repo, base, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			scope := scopePolicyScopeOf(jsonOut)
			Expect(scope).To(HaveKeyWithValue("policy", BeNil()), "the key is always present, null when no policy was used")

			text, stderr, exitCode := mode.run(repo, base)
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(text)).NotTo(MatchRegexp(`(?i)policy`))
		})

		It("#168 item 5: a clean committed policy under "+mode.name+" is disclosed as {path, digest, matches_head:true} and as same as HEAD in the text, with no WARNING", func() {
			repo, base := scopePolicyRepo(scopePolicyFile{coachConfigPath, singleRootPolicyJSON})

			jsonOut, stderr, exitCode := mode.run(repo, base, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(map[string]any{
				"path":         coachConfigPath,
				"digest":       digestOf(singleRootPolicyJSON),
				"matches_head": true,
				"cause":        diskCauseSameAsHead,
			})))

			text, stderr, exitCode := mode.run(repo, base)
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			expectDiskPolicyDisclosure(mode, text, diskPolicyLine(coachConfigPath, singleRootPolicyJSON, "same as HEAD."), "")
		})
	}

	type policyOnDisk struct {
		description string
		policies    []scopePolicyFile
		arrange     func(repo string)
		extraArgs   []string
		wantPath    string
		// usedBody is the policy bytes the scan used, whose digest scope.policy.digest carries.
		usedBody     string
		wantMatching bool
		wantCause    string
		wantLine     string
		wantWarning  string
	}
	const customPolicyPath = "policies/custom.json"
	scenarios := []policyOnDisk{
		{
			description: "a file rewritten with the same bytes (guard: only the content decides, not that the file was touched)",
			policies:    []scopePolicyFile{{coachConfigPath, singleRootPolicyJSON}},
			arrange:     func(repo string) { scopePolicyRewrite(repo, coachConfigPath, singleRootPolicyJSON) },
			wantPath:    coachConfigPath, usedBody: singleRootPolicyJSON, wantMatching: true, wantCause: diskCauseSameAsHead,
			wantLine: diskPolicyLine(coachConfigPath, singleRootPolicyJSON, "same as HEAD."),
		},
		{
			description: "a file edited on disk (guard: the digest alone differs, the path is the same)",
			policies:    []scopePolicyFile{{coachConfigPath, singleRootPolicyJSON}},
			arrange:     func(repo string) { scopePolicyRewrite(repo, coachConfigPath, reordered) },
			wantPath:    coachConfigPath, usedBody: reordered, wantMatching: false, wantCause: diskCauseDiffersFromHead,
			wantLine:    diskPolicyLine(coachConfigPath, reordered, "differs from HEAD."),
			wantWarning: diskPolicyDiffersWarning,
		},
		{
			description: "a policy named with --project-config and left alone",
			policies:    []scopePolicyFile{{customPolicyPath, singleRootPolicyJSON}},
			arrange:     func(string) {},
			extraArgs:   []string{"--project-config", customPolicyPath},
			wantPath:    customPolicyPath, usedBody: singleRootPolicyJSON, wantMatching: true, wantCause: diskCauseSameAsHead,
			wantLine: diskPolicyLine(customPolicyPath, singleRootPolicyJSON, "same as HEAD."),
		},
		{
			description: "a policy named with --project-config and edited on disk",
			policies:    []scopePolicyFile{{customPolicyPath, singleRootPolicyJSON}},
			arrange:     func(repo string) { scopePolicyRewrite(repo, customPolicyPath, reordered) },
			extraArgs:   []string{"--project-config", customPolicyPath},
			wantPath:    customPolicyPath, usedBody: reordered, wantMatching: false, wantCause: diskCauseDiffersFromHead,
			wantLine:    diskPolicyLine(customPolicyPath, reordered, "differs from HEAD."),
			wantWarning: diskPolicyDiffersWarning,
		},
	}
	for _, mode := range scopePolicyModes {
		for _, scenario := range scenarios {
			It("#168 item 5: scope.policy.matches_head is "+boolWord(scenario.wantMatching)+" for "+scenario.description+", under "+mode.name+", with its policy line, and a WARNING when the policy differs or is missing, in the text", func() {
				repo, base := scopePolicyRepo(scenario.policies...)
				scenario.arrange(repo)
				jsonArgs := append(append([]string{}, scenario.extraArgs...), "--format=json")

				jsonOut, stderr, exitCode := mode.run(repo, base, jsonArgs...)
				Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
				Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(map[string]any{
					"path":         scenario.wantPath,
					"digest":       digestOf(scenario.usedBody),
					"matches_head": scenario.wantMatching,
					"cause":        scenario.wantCause,
				})))

				text, stderr, exitCode := mode.run(repo, base, scenario.extraArgs...)
				Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
				expectDiskPolicyDisclosure(mode, text, scenario.wantLine, scenario.wantWarning)
			})
		}
	}
})

func boolWord(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
