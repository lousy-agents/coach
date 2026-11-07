package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A checkout whose line endings were converted (core.autocrlf, which is Git for Windows' default) holds different bytes
// from the committed blob with no edit at all: git status and git diff say it is clean. It is not a file with work in it,
// so a replacement is offered as for any other clean file, and the same bytes with a real edit are. The conversion is
// asked of git's own built-in line-ending, ident and encoding rules, which run no program; a file that a filter attribute
// selects a program for is not converted, and is taken to have work in it, because no program runs on a scan.
var _ = Describe("coach codesignal (Go): a committed policy that is invalid, in a checkout that converts line endings", func() {
	const policy = "cfg/p.json"
	var repo string

	// checkOutWithConversion turns core.autocrlf on and leaves the policy as a checkout would: the committed copy is
	// written by git itself, with its line endings converted, and any other document is written with them converted
	// by hand, as an edit made in an editor on Windows is.
	checkOutWithConversion := func(document string) {
		GinkgoHelper()
		gitIn(repo, "config", "core.autocrlf", "true")
		if document == invalidPolicy {
			Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())
			gitIn(repo, "checkout", "--", policy)
			return
		}
		writeWorktreeFile(repo, policy, strings.ReplaceAll(document, "\n", "\r\n"))
	}

	BeforeEach(func() {
		repo = commitGoRepoWithPolicy(policy, invalidPolicy)
	})

	It("offers the replacement when the checkout is the committed copy with its line endings converted", func() {
		checkOutWithConversion(invalidPolicy)
		Expect(gitIn(repo, "status", "--porcelain")).To(BeEmpty(), "git itself calls the checkout clean")

		_, stderr := scanExpectingRefusal(repo, "--project-config", policy)

		Expect(string(stderr)).NotTo(ContainSubstring("differs from the committed"), "nothing was edited:\n%s", stderr)
		Expect(lastLine(stderr)).To(ContainSubstring("replace the policy with it, commit, and rerun"), "stderr:\n%s", stderr)
	})

	It("names no cause and still offers a new policy in --check-project", func() {
		checkOutWithConversion(invalidPolicy)
		Expect(gitIn(repo, "status", "--porcelain")).To(BeEmpty())

		doc := expectCheckProject(repo, "--project-config", policy)

		Expect(causeOfGap(doc, "policy_invalid")).NotTo(Equal("changed_on_disk"), "gaps: %+v", doc.Gaps)
		Expect(nextActionKinds(doc)).To(ContainElement("author_policy"))
	})

	// An eol attribute converts the checkout as the setting does, and no filter and no edit is involved.
	It("offers the replacement when an eol attribute converted the checkout and nothing else changed", func() {
		Expect(os.WriteFile(filepath.Join(repo, ".git", "info", "attributes"), []byte(policy+" text eol=crlf\n"), 0o644)).To(Succeed())
		Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())
		gitIn(repo, "checkout", "--", policy)
		Expect(gitIn(repo, "status", "--porcelain")).To(BeEmpty(), "git itself calls the checkout clean")

		_, stderr := scanExpectingRefusal(repo, "--project-config", policy)

		Expect(string(stderr)).NotTo(ContainSubstring("differs from the committed"), "nothing was edited:\n%s", stderr)
		Expect(lastLine(stderr)).To(ContainSubstring("replace the policy with it, commit, and rerun"), "stderr:\n%s", stderr)
	})

	It("keeps the file when it holds a real edit as well as converted line endings", func() {
		checkOutWithConversion(validPolicyWithLayers)

		_, stderr := scanExpectingRefusal(repo, "--project-config", policy)

		Expect(string(stderr)).To(ContainSubstring("differs from the committed"), "stderr:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("replace the policy with it"), "stderr:\n%s", stderr)
	})

	// A policy the check calls changed on disk is one git status lists, so the report that names the cause also lists
	// the file among the changes it did not read.
	It("lists a policy it calls changed on disk among the changes of the working tree in --check-project", func() {
		checkOutWithConversion(validPolicyWithLayers)

		doc := expectCheckProject(repo, "--project-config", policy)

		Expect(causeOfGap(doc, "policy_invalid")).To(Equal("changed_on_disk"), "gaps: %+v", doc.Gaps)
		Expect(doc.DirtyWorktree.Paths).To(ContainElement(policy), "the cause names a file the report does not list as changed: %+v", doc.DirtyWorktree)
	})

	It("takes a file that a filter attribute selects a program for to have work in it, and never runs the program", func() {
		marks := GinkgoT().TempDir()
		clean := writeProbe(marks, "filter", filepath.Join(marks, "clean.sh"), "#!/bin/sh\n")
		gitIn(repo, "config", "filter.probe.clean", clean)
		Expect(os.WriteFile(filepath.Join(repo, ".git", "info", "attributes"), []byte(policy+" filter=probe\n"), 0o644)).To(Succeed())
		checkOutWithConversion(invalidPolicy)

		_, stderr := scanExpectingRefusal(repo, "--project-config", policy)

		Expect(string(stderr)).To(ContainSubstring("differs from the committed"), "stderr:\n%s", stderr)
		Expect(marksLeft(marks, "filter")).To(BeEmpty(), "the clean filter ran")
	})

	// The conversion asks git to hash the bytes as the policy's path, which must write no index, take no lock and run
	// neither the hook nor a filter that is defined for some other path.
	It("converts without writing the index, taking a lock, or running a hook or another path's filter", func() {
		marks := GinkgoT().TempDir()
		writeProbe(marks, "hook", filepath.Join(repo, ".git", "hooks", "post-index-change"), "#!/bin/sh\n")
		clean := writeProbe(marks, "filter", filepath.Join(marks, "clean.sh"), "#!/bin/sh\n")
		gitIn(repo, "config", "filter.probe.clean", clean)
		Expect(os.WriteFile(filepath.Join(repo, ".git", "info", "attributes"), []byte("*.other filter=probe\n"), 0o644)).To(Succeed())
		checkOutWithConversion(invalidPolicy)
		Expect(gitIn(repo, "status", "--porcelain")).To(BeEmpty())
		later := time.Now().Add(time.Hour)
		Expect(os.Chtimes(filepath.Join(repo, policy), later, later)).To(Succeed())
		Expect(os.Remove(filepath.Join(marks, "hook.ran"))).To(Or(Succeed(), MatchError(os.ErrNotExist)))
		before := indexAsItIs(repo)

		_, stderr := scanExpectingRefusal(repo, "--project-config", policy)

		Expect(indexAsItIs(repo)).To(Equal(before), "the scan wrote the index")
		Expect(marksLeft(marks, "hook")).To(BeEmpty(), "the post-index-change hook ran")
		Expect(marksLeft(marks, "filter")).To(BeEmpty(), "a filter ran")
		Expect(filepath.Join(repo, ".git", "index.lock")).NotTo(BeAnExistingFile())
		Expect(lastLine(stderr)).To(ContainSubstring("replace the policy with it, commit, and rerun"), "stderr:\n%s", stderr)
	})
})
