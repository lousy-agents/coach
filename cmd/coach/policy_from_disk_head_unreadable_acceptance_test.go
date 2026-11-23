package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

const diskCauseHeadUnreadable = "head_unreadable"

// ADR-009 decision 6: the committed copy is read only for disclosure, so a committed copy that cannot be read does not stop a
// valid policy on disk. A committed copy that is read and is not usable (invalid, a link, a directory) is differs_from_head.
var _ = Describe("coach codesignal reads the project policy from disk: a policy at HEAD that cannot be read (#168 A2b)", func() {
	const customPolicyPath = "policies/custom.json"

	oversized := strings.Repeat(" ", int(projectconfig.MaxBytes)+1)
	headUnreadableLine := func(path, body string) string {
		return regexp.QuoteMeta(diskPolicyLine(path, body, "")) + `.*`
	}
	removeCommittedObject := func(repo, path string) {
		oid := strings.TrimSpace(gitIn(repo, "rev-parse", "HEAD:"+path))
		object := filepath.Join(repo, ".git", "objects", oid[:2], oid[2:])
		Expect(os.Rename(object, object+".orig")).To(Succeed(), "the object must be loose: a fresh commit writes it so")
		DeferCleanup(func() { _ = os.Rename(object+".orig", object) })
	}
	expectUsedWithUnreadableHead := func(mode scopePolicyMode, repo, base, path, body string, args ...string) {
		GinkgoHelper()
		_, jsonOut := diskPolicyJSONScan(mode, repo, base, args...)
		Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(path, body, diskCauseHeadUnreadable))))

		text := diskPolicyTextScan(mode, repo, base, args...)

		after := diskPolicyLinesAfterSummary(mode, text, 2)
		Expect(after).To(HaveLen(2), "text:\n%s", text)
		Expect(after[0]).To(MatchRegexp("^" + headUnreadableLine(path, body) + "$"))
		Expect(after[0]).NotTo(HaveSuffix("same as HEAD."))
		Expect(after[1]).To(MatchRegexp(`^WARNING: .*HEAD.*could not be read`), "text:\n%s", text)
		Expect(after[1]).To(ContainSubstring(path))
		Expect(after[1]).NotTo(ContainSubstring("differs from HEAD's"), "the committed copy was not compared, so it is not said to differ")
	}

	for _, mode := range scopePolicyModes {
		for _, subject := range []struct {
			description, path string
			args              []string
		}{
			{"a discovered canonical policy", coachConfigPath, nil},
			{"the canonical policy named with --project-config", coachConfigPath, []string{"--project-config", coachConfigPath}},
			{"a named policy", customPolicyPath, []string{"--project-config", customPolicyPath}},
		} {
			It("#168 A2b: "+subject.description+" committed over the size budget, beside a valid policy on disk, is used under "+mode.name+" with the cause head_unreadable", func() {
				repo, base := diskPolicyRepo(scopePolicyFile{subject.path, oversized})
				scopePolicyRewrite(repo, subject.path, goLayerPolicyConfigJSON)

				expectUsedWithUnreadableHead(mode, repo, base, subject.path, goLayerPolicyConfigJSON, subject.args...)
			})
		}

		It("#168 A2b: a policy committed over the size budget and fixed on disk to one with no violation qualifies the all-clear under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, oversized})
			scopePolicyRewrite(repo, coachConfigPath, diskPolicyWithoutForbiddenImports)

			text := diskPolicyTextScan(mode, repo, base)

			Expect(string(text)).To(MatchRegexp(`(?m)^No active CodeSignal findings, but .*HEAD.*\.$`))
			Expect(string(text)).NotTo(MatchRegexp(`(?m)^No active CodeSignal findings\.$`))
		})

		It("#168 A2b: a policy committed over the size budget and over the size budget on disk is refused for the disk under "+mode.name+", with no head_unreadable report", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, oversized})
			scopePolicyRewrite(repo, coachConfigPath, oversized+" ")

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

			expectDiskJudgedRefusal(stdout, stderr, exitCode, coachConfigPath, "larger than the policy size limit")
		})

		// Baseline-affected on git 2.43: its missing-object rows fail in the same way, so this row is red there for the
		// object, not for the behavior, until the sandbox's git is newer.
		It("#168 A2b: a policy whose committed object is missing, beside a valid policy on disk, is used under "+mode.name+" with the cause head_unreadable", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, `{"schema_version":"1","roots":["."],"note":"the committed copy"}`})
			removeCommittedObject(repo, coachConfigPath)
			scopePolicyRewrite(repo, coachConfigPath, goLayerPolicyConfigJSON)

			expectUsedWithUnreadableHead(mode, repo, base, coachConfigPath, goLayerPolicyConfigJSON)
		})
	}
})
