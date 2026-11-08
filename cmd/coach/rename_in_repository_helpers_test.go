package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// renameInTheRepository takes the step the advice describes in words: the committed file is renamed, in a directory
// made for the new name, and the rename is staged for the commit that follows.
func renameInTheRepository(repo, from, to string) {
	GinkgoHelper()
	Expect(os.MkdirAll(filepath.Join(repo, filepath.Dir(to)), 0o755)).To(Succeed())
	gitIn(repo, "mv", "--", from, to)
}
