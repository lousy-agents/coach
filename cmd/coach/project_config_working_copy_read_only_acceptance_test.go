package main

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A scan reads. Deciding whether the file at the path of a committed policy has work in it is done by comparing its
// bytes with the committed copy's, read through the repository root, and not by asking git for a diff: a diff against the
// working tree refreshes the index and writes it back, takes index.lock, runs the repository's post-index-change hook and
// runs its clean filter over a file whose stat is dirty. The probe is a hook and a filter that each leave a mark, and a
// policy whose stat is dirty because its modification time moved while its bytes did not.
var _ = Describe("coach codesignal (Go): deciding whether a committed policy that is invalid has work in it is read-only", func() {
	const policy = "cfg/p.json"
	var (
		repo, marks string
	)

	BeforeEach(func() {
		repo = commitGoRepoWithPolicy(policy, invalidPolicy)
		marks = GinkgoT().TempDir()
		writeProbe(marks, "hook", filepath.Join(repo, ".git", "hooks", "post-index-change"), "#!/bin/sh\n")
		cleanFilter := writeProbe(marks, "filter", filepath.Join(marks, "clean.sh"), "#!/bin/sh\n")
		gitIn(repo, "config", "filter.probe.clean", cleanFilter)
		Expect(os.WriteFile(filepath.Join(repo, ".git", "info", "attributes"), []byte("*.json filter=probe\n"), 0o644)).To(Succeed())
		later := time.Now().Add(time.Hour)
		Expect(os.Chtimes(filepath.Join(repo, policy), later, later)).To(Succeed())
	})

	It("writes no index, takes no lock, and runs neither the hook nor the filter", func() {
		before := indexAsItIs(repo)

		_, stderr := scanExpectingRefusal(repo, "--project-config", policy)

		Expect(indexAsItIs(repo)).To(Equal(before), "the scan wrote the index")
		Expect(marksLeft(marks, "hook")).To(BeEmpty(), "the post-index-change hook ran")
		Expect(marksLeft(marks, "filter")).To(BeEmpty(), "the clean filter ran")
		Expect(filepath.Join(repo, ".git", "index.lock")).NotTo(BeAnExistingFile())
		Expect(lastLine(stderr)).To(ContainSubstring("replace the policy with it"), "the file holds the committed bytes, so the replacement is still offered:\n%s", stderr)
	})
})

// writeProbe writes a script that records that it ran, in a file under marks named for what it is, and then does what
// body says. It returns the script's path.
func writeProbe(marks, name, path, body string) string {
	GinkgoHelper()
	Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	mark := filepath.Join(marks, name+".ran")
	Expect(os.WriteFile(path, []byte(body+"echo ran >> '"+mark+"'\n"+scriptTail(name)), 0o755)).To(Succeed())
	return path
}

// scriptTail is what a probe does after it leaves its mark: a filter passes its input through.
func scriptTail(name string) string {
	if name == "filter" {
		return "cat\n"
	}
	return ""
}

func marksLeft(marks, name string) string {
	GinkgoHelper()
	content, err := os.ReadFile(filepath.Join(marks, name+".ran"))
	if os.IsNotExist(err) {
		return ""
	}
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}

// indexAsItIs is the index file's bytes and modification time, which any write to it changes.
func indexAsItIs(repo string) string {
	GinkgoHelper()
	path := filepath.Join(repo, ".git", "index")
	info, err := os.Stat(path)
	Expect(err).NotTo(HaveOccurred())
	content, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return info.ModTime().String() + "\x00" + string(content)
}
