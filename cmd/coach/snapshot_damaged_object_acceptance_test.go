package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A loose object that is there and cannot be read is damaged, not missing. `git cat-file --batch` prints "<id>
// missing" for it and its error on stderr, and the snapshot told the user that the object was not in the local
// object store and that Coach does not fetch objects, with the way to fetch it: the wrong remedy, since the object
// is in the store and fetching does not replace it. v0.11.0 surfaced git's own error.
var _ = Describe("coach codesignal: a damaged object is reported as damaged, not as missing", func() {
	It("names the damage and git's words for it, and offers no fetch", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		object := strings.TrimSpace(gitOutputIn(repo, "rev-parse", "HEAD:cfg/p.json"))
		loose := filepath.Join(repo, ".git", "objects", object[:2], object[2:])
		Expect(os.Chmod(loose, 0o644)).To(Succeed())
		Expect(os.WriteFile(loose, []byte("garbage"), 0o644)).To(Succeed())

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("git cannot read"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("damaged"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring(`"cfg/p.json"`), "the first path is named:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("inflate"), "git's own words are quoted:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("not in the local object store"), "the object is in the store:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("fetch"), "fetching does not replace a damaged object:\n%s", stderr)
	})
})
