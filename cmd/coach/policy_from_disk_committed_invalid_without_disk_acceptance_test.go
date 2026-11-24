package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

type invalidCommittedPolicy struct {
	description string
	commit      func(repo string)
}

// A discovered policy that is deleted on disk is never read from the commit, however unusable the commit's copy is
// (#168 A3, ADR-009): the scan uses a root project.json from disk or runs file-local, and says which policy is absent.
var _ = Describe("coach codesignal reads a discovered policy from disk only: a committed canonical policy that is not usable and is deleted on disk (#168 A3)", func() {
	invalidCommits := []invalidCommittedPolicy{
		{"invalid JSON", func(repo string) { commitFile(repo, coachConfigPath, "{bad") }},
		{"schema-invalid", func(repo string) { commitFile(repo, coachConfigPath, `{"schema_version":"1","layers":"not a list"}`) }},
		{"over the size limit", func(repo string) {
			commitFile(repo, coachConfigPath, strings.Repeat(" ", int(projectconfig.MaxBytes)+1))
		}},
		{"a directory", func(repo string) { commitFile(repo, coachConfigPath+"/inner.json", singleRootPolicyJSON) }},
		{"a link", func(repo string) {
			symlinkAt(repo, coachConfigPath)
			gitIn(repo, "add", coachConfigPath)
			gitInWithIdentity(repo, "commit", "-q", "-m", "a policy as a link")
		}},
	}
	for _, mode := range scopePolicyModes {
		for _, invalid := range invalidCommits {
			It("#168 A3: committed as "+invalid.description+" is file-local under "+mode.name+" with the absent-policy WARNING when nothing is on disk", func() {
				repo, base := diskPolicyRepo()
				invalid.commit(repo)
				removeTree(repo, coachConfigPath)

				report, jsonOut := diskPolicyJSONScan(mode, repo, base)

				Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", BeNil()))
				expectDiagnosticKind(report, "project_policy_absent_on_disk", coachConfigPath)
				Expect(warningLinesNaming(diskPolicyTextScan(mode, repo, base), coachConfigPath)).NotTo(BeEmpty())
			})
		}

		It("#168 A3: committed as invalid JSON is replaced under "+mode.name+" by an uncommitted root project.json on disk, as the deprecated fallback", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, "{bad"})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			scopePolicyRewrite(repo, "project.json", goLayerPolicyConfigJSON)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "only project.json forbids handlers -> db")
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(legacyConfigPath, goLayerPolicyConfigJSON, diskCauseNotCommitted))))
			expectDiagnosticKind(report, "project_config_deprecated", legacyConfigPath)
			expectDiagnosticKind(report, "project_policy_absent_on_disk", coachConfigPath)
		})
	}
})
