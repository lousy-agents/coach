package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A restore brings back what is committed. A committed policy that is not valid, or is empty, and is gone
// from disk is answered with a new policy in both modes, at once, and not with a restore that brings back a
// file the next run refuses; an empty committed file is not described as an empty index entry.
var _ = Describe("coach codesignal: a committed policy that is not valid, gone from disk", func() {
	DescribeTable("is answered with a new policy and no restore",
		func(content string) {
			repo := commitScannableGoRepo()
			commitFile(repo, "cfg/p.json", content)
			removeTree(repo, "cfg")

			stderr := scanExplicitPolicy(repo, "cfg/p.json")
			doc := expectCheckProject(repo, "--project-config", "cfg/p.json")

			Expect(string(stderr)).To(ContainSubstring("committed copy is not a valid policy"), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("--suggest-project-config"), "a new policy is offered:\n%s", stderr)
			Expect(printedRestoreCommand(stderr)).To(BeEmpty(), "no restore of a copy the scan refuses:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("empty entry"), "the commit holds the content, not the index:\n%s", stderr)
			Expect(nextActionKinds(doc)).To(ContainElement("author_policy"), "next actions: %+v", doc.NextActions)
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"), "next actions: %+v", doc.NextActions)
		},
		Entry("garbage", "{"),
		Entry("an empty file", ""),
		Entry("valid JSON of the wrong shape", `{"schema_version":"1"}`),
	)
})
