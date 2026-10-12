package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

// corruptCommittedBlob deletes path's loose object file after it has been
// committed, leaving the commit/tree objects (and thus revision resolution)
// intact while making the blob itself unreadable.
func corruptCommittedBlob(repo, path string) {
	revCmd := exec.Command("git", "rev-parse", "HEAD:"+path)
	revCmd.Dir = repo
	output, err := revCmd.Output()
	Expect(err).NotTo(HaveOccurred())
	blobSHA := strings.TrimSpace(string(output))
	Expect(blobSHA).To(HaveLen(40))

	objectPath := filepath.Join(repo, ".git", "objects", blobSHA[:2], blobSHA[2:])
	_, statErr := os.Stat(objectPath)
	Expect(statErr).NotTo(HaveOccurred(), "expected a loose object at %s -- was the fixture repo gc'd?", objectPath)
	Expect(os.Remove(objectPath)).To(Succeed())
}

// corruptCommittedTree deletes dirPath's own subtree loose object after it
// has been committed, leaving the parent tree/commit objects (and thus
// revision resolution) intact while making a path underneath dirPath
// unresolvable. Unlike corruptCommittedBlob (which corrupts the leaf blob
// itself), this exercises a git-plumbing call that only walks tree objects
// without ever opening blob content.
func corruptCommittedTree(repo, dirPath string) {
	revCmd := exec.Command("git", "rev-parse", "HEAD:"+dirPath)
	revCmd.Dir = repo
	output, err := revCmd.Output()
	Expect(err).NotTo(HaveOccurred())
	treeSHA := strings.TrimSpace(string(output))
	Expect(treeSHA).To(HaveLen(40))

	objectPath := filepath.Join(repo, ".git", "objects", treeSHA[:2], treeSHA[2:])
	_, statErr := os.Stat(objectPath)
	Expect(statErr).NotTo(HaveOccurred(), "expected a loose object at %s -- was the fixture repo gc'd?", objectPath)
	Expect(os.Remove(objectPath)).To(Succeed())
}
