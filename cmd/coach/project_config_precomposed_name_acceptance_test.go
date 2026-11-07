package main

import (
	"bytes"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	precomposedPolicy = "cfg/pé.json"  // é as one code point, the spelling git stores on macOS
	decomposedPolicy  = "cfg/pé.json" // e and a combining accent, the spelling a file system hands back
)

// commitDecomposedPolicy commits a policy whose name is decomposed and deletes it from disk, and skips
// where git does not precompose the names it lists (git on macOS does by default; others never do).
func commitDecomposedPolicy(repo string) {
	commitFile(repo, decomposedPolicy, singleRootPolicyJSON)
	listed := gitIn(repo, "ls-files", "-z", "--", "cfg")
	if !bytes.Contains([]byte(listed), []byte(precomposedPolicy)) {
		Skip("this git stores the name as it was given, so there is no second spelling of it")
	}
	ExpectWithOffset(1, os.Remove(filepath.Join(repo, decomposedPolicy))).To(Succeed())
}

// git on macOS stores and lists a name precomposed, whichever form it was given in. A committed policy
// named in the other form and deleted from disk is still the policy the index holds, so both modes
// offer the restore for it; --check-project used to exit 1 and the scan blamed git for Coach's own
// refusal to read the listing.
var _ = Describe("coach codesignal: a policy named in another Unicode form than git stores it", func() {
	It("offers the restore in a scan and in --check-project, and blames no git error", func() {
		repo := commitScannableGoRepo()
		commitDecomposedPolicy(repo)

		doc := expectCheckProject(repo, "--project-config", decomposedPolicy)
		stderr := scanExplicitPolicy(repo, decomposedPolicy)

		Expect(nextActionKinds(doc)).To(ContainElement("restore_policy"), "next actions: %+v", doc.NextActions)
		Expect(string(stderr)).NotTo(ContainSubstring("git said"), "stderr:\n%s", stderr)
		Expect(printedRestoreCommand(stderr)).To(HavePrefix("git restore --source="), "stderr:\n%s", stderr)
	})

	It("leaves the policy's own path out of the dirty worktree in either spelling", func() {
		repo := commitScannableGoRepo()
		commitDecomposedPolicy(repo)

		typedDecomposedDoc := expectCheckProject(repo, "--project-config", decomposedPolicy)
		typedPrecomposedDoc := expectCheckProject(repo, "--project-config", precomposedPolicy)

		Expect(typedDecomposedDoc.DirtyWorktree).To(Equal(typedPrecomposedDoc.DirtyWorktree), "the same state, typed in the other form")
		Expect(typedDecomposedDoc.DirtyWorktree.Paths).NotTo(ContainElement(precomposedPolicy))
	})
})
