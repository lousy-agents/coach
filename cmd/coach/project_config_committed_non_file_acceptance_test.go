package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// commitLinkThenWriteFile leaves a regular file on disk at a path the revision commits as a link: the
// state of a user who replaced the link with a copy of what it pointed at and has not committed it.
func commitLinkThenWriteFile(repo, path string) {
	symlinkAt(repo, path)
	gitIn(repo, "add", path)
	gitInWithIdentity(repo, "commit", "-q", "-m", "a policy as a link")
	ExpectWithOffset(1, os.Remove(filepath.Join(repo, path))).To(Succeed())
	writeWorktreeFile(repo, path, singleRootPolicyJSON)
}

// commitDirectoryThenWriteFile leaves a regular file on disk at a path the revision holds files under.
func commitDirectoryThenWriteFile(repo, path string) {
	commitFile(repo, path+"/inner.json", singleRootPolicyJSON)
	removeTree(repo, path)
	writeWorktreeFile(repo, path, singleRootPolicyJSON)
}

type notAFileSpec struct {
	arrange func(repo, path string)
	policy  string
	named   string
	cause   string
}

// What the revision holds at a policy path is read before its content is, so a link or a directory
// is called one by a scan and by --check-project alike. Both used to blame the document or call the
// path missing, and --check-project offered a new policy for a path the revision already holds.
var _ = Describe("coach codesignal: a policy the revision commits as something other than a file", func() {
	DescribeTable("is named as that, and no new policy is offered over it",
		func(spec notAFileSpec) {
			repo := commitScannableGoRepo()
			spec.arrange(repo, spec.policy)

			stderr := scanExplicitPolicy(repo, spec.policy)
			doc := expectCheckProject(repo, "--project-config", spec.policy)

			Expect(string(stderr)).To(ContainSubstring("committed as a "+spec.named), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("not valid JSON"), "the content was never the fault:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("--suggest-project-config"), "stderr:\n%s", stderr)
			Expect(causeOfGap(doc, "policy_invalid")).To(Equal(spec.cause), "gaps: %+v", doc.Gaps)
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"), "next actions: %+v", doc.NextActions)
		},
		Entry("a symbolic link, with a regular file on disk", notAFileSpec{commitLinkThenWriteFile, "cfg/p.json", "symbolic link", "committed_as_link"}),
		Entry("a directory, with a regular file on disk", notAFileSpec{commitDirectoryThenWriteFile, "cfg", "directory", "directory"}),
	)
})
