package codesignalcli

import (
	"io/fs"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func body_projectSnapshotAcceptanceTest_listsEveryFileTrackedAtRevisionMatchingGitLsTree_62(dir string, sha string) {
	fsys, err := NewGoSnapshotFS(dir, sha)
	Expect(err).NotTo(HaveOccurred())

	var got []string
	Expect(fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, walkErr error) error {
		Expect(walkErr).NotTo(HaveOccurred())
		if !d.IsDir() {
			got = append(got, p)
		}
		return nil
	})).To(Succeed())
	sort.Strings(got)

	Expect(got).To(Equal(snapshotGroundTruthLsTree(dir, sha)))
}

func body_projectSnapshotAcceptanceTest_visitsEveryTrackedFileExactlyOnceInLexicalOrderV_88(dir string, sha string) {
	fsys, err := NewGoSnapshotFS(dir, sha)
	Expect(err).NotTo(HaveOccurred())

	var visited []string
	Expect(fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, walkErr error) error {
		Expect(walkErr).NotTo(HaveOccurred())
		if !d.IsDir() {
			visited = append(visited, p)
		}
		return nil
	})).To(Succeed())

	sorted := append([]string(nil), visited...)
	sort.Strings(sorted)
	Expect(visited).To(Equal(sorted), "fs.WalkDir must visit files in lexical order")
	Expect(visited).To(ConsistOf("README.md", "main.go", "pkg/lib.go"))
}

func body_projectSnapshotAcceptanceTest_statsAFileSSizeWithoutEverRunningGitShowMustNotB_140(dir string, sha string) {
	var invokedArgs [][]string
	originalRunner := runSnapshotGit
	DeferCleanup(func() { runSnapshotGit = originalRunner })
	runSnapshotGit = func(d string, maxStdout, maxStderr int64, timeout time.Duration, args ...string) ([]byte, error) {
		invokedArgs = append(invokedArgs, append([]string(nil), args...))
		return originalRunner(d, maxStdout, maxStderr, timeout, args...)
	}

	fsys, err := NewGoSnapshotFS(dir, sha)
	Expect(err).NotTo(HaveOccurred())
	invokedArgs = nil // discard construction's own ls-tree call; this spec only cares about Stat

	info, err := fs.Stat(fsys, "pkg/lib.go")
	Expect(err).NotTo(HaveOccurred())
	want := snapshotGroundTruthShow(dir, sha, "pkg/lib.go")
	Expect(info.Size()).To(Equal(int64(len(want))), "Stat's reported size must match the blob's real content length")

	for _, args := range invokedArgs {
		Expect(args).NotTo(ContainElement("show"), "fs.Stat must never invoke git show (it would buffer the whole blob just to size it); invoked git args: %v", invokedArgs)
	}
}
