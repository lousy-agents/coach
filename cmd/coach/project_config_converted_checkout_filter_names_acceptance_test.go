package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// git check-attr prints a string value as it is written, so a filter driver that is named unset or unspecified prints what an
// attribute that is unset, or not set at all, prints. A scan that read those two words as "no filter" asked git to hash the
// bytes, and git ran the driver's program from the repository's configuration: a program the scan is documented never to run,
// and one whose output decides the hash, so a clean filter that prints the committed blob hides an edit. The attribute is asked
// for in full, so that a filter attribute with any value, and the unset one as well, is a file taken to have work in it.
var _ = Describe("coach codesignal (Go): a committed invalid policy, in a checkout that converts line endings, with a filter attribute spelled like no filter", func() {
	const policy = "cfg/p.json"

	// convertedCheckout leaves the policy as a checkout on Windows would, with its line endings converted by git itself. The
	// filter is configured afterwards: git checkout would run a filter that is already selected for the path, and the spec is
	// about what the scan runs.
	convertedCheckout := func(repo string) {
		GinkgoHelper()
		gitIn(repo, "config", "core.autocrlf", "true")
		Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())
		gitIn(repo, "checkout", "--", policy)
		Expect(gitIn(repo, "status", "--porcelain")).To(BeEmpty(), "git itself calls the checkout clean")
	}
	selectFilter := func(repo, attributes string) {
		GinkgoHelper()
		Expect(os.WriteFile(filepath.Join(repo, ".git", "info", "attributes"), []byte(attributes), 0o644)).To(Succeed())
	}

	DescribeTable("takes the file to have work in it, and runs no program of the filter's name",
		func(driver, program string) {
			repo := commitGoRepoWithPolicy(policy, invalidPolicy)
			marks := GinkgoT().TempDir()
			convertedCheckout(repo)
			gitIn(repo, "config", "filter."+driver+"."+program, writeProbe(marks, "filter", filepath.Join(marks, "program.sh"), "#!/bin/sh\n"))
			selectFilter(repo, policy+" filter="+driver+"\n")

			_, stderr := scanExpectingRefusal(repo, "--project-config", policy)

			Expect(marksLeft(marks, "filter")).To(BeEmpty(), "the %s program of the filter named %q ran", program, driver)
			Expect(string(stderr)).To(ContainSubstring("differs from the committed"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("replace the policy with it"), "stderr:\n%s", stderr)
		},
		Entry("a filter named unset, with a clean program", "unset", "clean"),
		Entry("a filter named unset, with a process program", "unset", "process"),
		Entry("a filter named unspecified, with a clean program", "unspecified", "clean"),
		Entry("a filter named unspecified, with a process program", "unspecified", "process"),
	)

	// git check-attr prints the attribute that is unset as it prints the driver named unset. Telling them apart is not
	// possible, and the safe reading of both is a file that may have work in it.
	It("takes a file whose filter attribute is unset to have work in it as well", func() {
		repo := commitGoRepoWithPolicy(policy, invalidPolicy)
		convertedCheckout(repo)
		selectFilter(repo, policy+" -filter\n")

		_, stderr := scanExpectingRefusal(repo, "--project-config", policy)

		Expect(string(stderr)).To(ContainSubstring("differs from the committed"), "stderr:\n%s", stderr)
	})

	// The control: a driver that is configured under those names but that no attribute selects is no filter at all, and the
	// converted checkout still gets the way out.
	It("still offers the replacement when no filter attribute selects the driver configured under those names", func() {
		repo := commitGoRepoWithPolicy(policy, invalidPolicy)
		marks := GinkgoT().TempDir()
		convertedCheckout(repo)
		for _, driver := range []string{"unset", "unspecified"} {
			gitIn(repo, "config", "filter."+driver+".clean", writeProbe(marks, "filter", filepath.Join(marks, driver+".sh"), "#!/bin/sh\n"))
		}

		_, stderr := scanExpectingRefusal(repo, "--project-config", policy)

		Expect(marksLeft(marks, "filter")).To(BeEmpty(), "a filter no attribute selects ran")
		Expect(string(stderr)).NotTo(ContainSubstring("differs from the committed"), "nothing was edited:\n%s", stderr)
		Expect(lastLine(stderr)).To(ContainSubstring("replace the policy with it, commit, and rerun"), "stderr:\n%s", stderr)
	})
})
