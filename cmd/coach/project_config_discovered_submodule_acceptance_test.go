package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A policy Coach would discover at .lousy-agents/coach.json cannot exist when .lousy-agents is a submodule: the
// superproject records a submodule as one commit, so a file inside it can be neither committed nor added here.
// The scan and --check-project say so for it, as they do for a policy named with --project-config, whether the
// checkout is there or has been removed, and neither tells the user to write a policy that git would refuse to add.
var _ = Describe("coach codesignal: a discovered policy under a submodule at .lousy-agents, in the scan and in --check-project", func() {
	It("words a file in the checkout as the submodule's and not as one to commit, and --check-project names the same cause", func() {
		repo := commitScannableGoRepo()
		addSubmoduleAt(repo, ".lousy-agents")

		diagnostics := scanDiagnosticsAtTheCanonicalPath(repo)
		doc := expectCheckProject(repo)

		Expect(diagnostics).To(HaveLen(1), "diagnostics: %v", diagnostics)
		Expect(diagnostics[0]).To(HavePrefix("project_config_unusable "), "the file cannot be committed here, so it is not 'not committed': %s", diagnostics[0])
		Expect(causeInTheDiagnostic.FindStringSubmatch(diagnostics[0])).To(Equal([]string{"cause=in_submodule", "in_submodule"}), "the cause the check names: %s", diagnostics[0])
		Expect(diagnostics[0]).To(ContainSubstring(`".lousy-agents"`), "the submodule is named: %s", diagnostics[0])
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
