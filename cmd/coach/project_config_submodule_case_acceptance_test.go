package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The index records a submodule in one spelling, and a repository whose git ignores case treats a name in another case as
// the same path. A policy named below the submodule in another case than it is recorded is inside it all the same: it
// can be neither restored nor committed here, so the scan and the project check say so, name the submodule as the index
// records it, and offer no new policy, which would be written into the submodule's directory and could not be committed
// either. Setting core.ignorecase makes the rows the same on a disk that folds case and one that does not.
var _ = Describe("coach codesignal: a policy named below a submodule in another case than the index records it", func() {
	const recorded = "vendor/sub"

	DescribeTable("names the submodule as recorded, and offers no new policy under it",
		func(typed string, deinit bool) {
			repo := commitScannableGoRepo()
			addSubmoduleAt(repo, recorded)
			if deinit {
				gitIn(repo, "submodule", "deinit", "-f", recorded)
				removeTree(repo, recorded)
			}
			gitIn(repo, "config", "core.ignorecase", "true")

			stderr := scanExplicitPolicy(repo, typed)
			doc := expectCheckProject(repo, "--project-config", typed)

			Expect(string(stderr)).To(ContainSubstring(`"`+recorded+`"`), "the scan names the submodule as the index records it; stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("--output"), "a new policy under a submodule cannot be committed; stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("--suggest-project-config"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("commit the file"), "the file cannot be committed here; stderr:\n%s", stderr)
			Expect([]string{causeOfGap(doc, "policy_missing"), causeOfGap(doc, "policy_invalid")}).To(ContainElement("in_submodule"), "gaps: %+v", doc.Gaps)
			_, authors := nextActionOfKind(doc, "author_policy")
			Expect(authors).To(BeFalse(), "next actions: %+v", doc.NextActions)
		},
		Entry("the submodule's directory in another case, checkout present", "vendor/Sub/coach.json", false),
		Entry("a directory above it in another case, checkout present", "Vendor/sub/coach.json", false),
		Entry("the whole path in another case, checkout present", "VENDOR/SUB/cfg/p.json", false),
		Entry("the submodule's directory in another case, checkout removed", "vendor/Sub/coach.json", true),
		Entry("a directory above it in another case, checkout removed", "Vendor/sub/coach.json", true),
	)
})

var _ = Describe("the README: a submodule named in another case than the index records it", func() {
	It("says the name is inside the submodule where git ignores case, and the message names the submodule as recorded", func() {
		bullet := readmeBullet("A policy inside a submodule checkout")
		text := strings.Join(strings.Fields(bullet), " ")

		Expect(text).To(ContainSubstring("A name that spells the submodule's own path in another case than the index records it, where git ignores case (`core.ignorecase`), is inside it all the same, and the message names the submodule as the index records it."))
	})
})
