package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A policy Coach would discover at .lousy-agents/coach.json cannot exist when .lousy-agents is a submodule: the
// superproject records a submodule as one commit, so a file inside it can be neither committed nor added here.
// A file in the checkout is an unusable canonical entry, so the scan refuses it as it does a policy named with
// --project-config, and --check-project names the same cause. The halves diverge until #168 A2b-2 moves the check to disk.
// Neither tells the user to write a policy that git would refuse to add.
var _ = Describe("coach codesignal: a discovered policy under a submodule at .lousy-agents, in the scan and in --check-project (#168 A2b)", func() {
	It("refuses the scan for a file in the checkout, naming the submodule as the cause, and --check-project names the same cause", func() {
		repo := commitScannableGoRepo()
		addSubmoduleAt(repo, ".lousy-agents")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")
		doc := expectCheckProject(repo)

		Expect(exitCode).To(Equal(2), "the canonical entry lies inside a submodule checkout, so the scan refuses; stdout=%s stderr=%s", stdout, stderr)
		Expect(stdout).To(BeEmpty(), "no report on a refused policy")
		Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
		Expect(string(stderr)).To(ContainSubstring(coachConfigPath))
		Expect(string(stderr)).To(ContainSubstring("in_submodule"), "the cause the check names")
		Expect(string(stderr)).NotTo(ContainSubstring("project_config_unusable"), "no continuing without the policy")
		Expect(causeOfGap(doc, "policy_missing")).To(Equal("in_submodule"), "the check's gaps: %+v", doc.Gaps)
	})

	It("names the submodule in --check-project once the checkout is removed, and offers no policy to write", func() {
		repo := commitScannableGoRepo()
		addSubmoduleAt(repo, ".lousy-agents")
		gitIn(repo, "submodule", "deinit", "-f", ".lousy-agents")

		doc := expectCheckProject(repo)

		Expect(causeOfGap(doc, "policy_missing")).To(Equal("in_submodule"), "the check's gaps: %+v", doc.Gaps)
		Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"), "git refuses to add a file inside the submodule, so writing one leads nowhere")
	})

	It("says nothing about a policy in the scan once the checkout is removed, and does not tell the user to commit one", func() {
		repo := commitScannableGoRepo()
		addSubmoduleAt(repo, ".lousy-agents")
		gitIn(repo, "submodule", "deinit", "-f", ".lousy-agents")

		Expect(scanDiagnosticsAtTheCanonicalPath(repo)).To(BeEmpty(), "nothing is on disk, so the scan has nothing to say about the path")
	})
})
