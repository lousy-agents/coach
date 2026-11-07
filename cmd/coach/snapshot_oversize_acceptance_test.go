//go:build unix

package main

import (
	"io"
	"os/exec"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// zeroReader is an endless stream of zero bytes: a committed file of any size without writing one to disk.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// commitBigBlob commits a file of size zero bytes at path. The object is made by git from a stream, so the
// working tree never holds it, and git compresses it to a few hundred KiB.
func commitBigBlob(repo, path string, size int64) {
	hash := exec.Command("git", "hash-object", "-w", "--stdin")
	hash.Dir = repo
	hash.Stdin = io.LimitReader(zeroReader{}, size)
	output, err := hash.Output()
	Expect(err).NotTo(HaveOccurred(), "git hash-object")
	oid := strings.TrimSpace(string(output))

	add := exec.Command("git", "update-index", "--add", "--cacheinfo", "100644,"+oid+","+path)
	add.Dir = repo
	combined, err := add.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git update-index: %s", combined)

	commit := exec.Command("git", "commit", "-m", "commit "+path+" of "+strconv.FormatInt(size, 10)+" bytes")
	commit.Dir = repo
	commit.Env = commitEnv
	combined, err = commit.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git commit: %s", combined)
}

// The source snapshot is built from every file of the revision, and a file past the limit on one file is
// not read into memory or written. It was refused, and the refusal ended the scan, although the scan reads
// no such file: v0.11.0 scanned a repository with a large asset. The scan now goes on without the bytes.
var _ = Describe("coach codesignal: a committed file past the limit on one file of the snapshot", func() {
	const (
		// past the largest limit the snapshot has ever had (512 MiB)
		hugeAsset = 513 << 20
		// past the limit on one file (32 MiB), and cheap
		bigFile = 33 << 20
	)

	It("scans a repository with a huge asset that no analyzer reads, as v0.11.0 did", func() {
		repo := commitScannableGoRepo()
		commitBigBlob(repo, "assets/blob.bin", hugeAsset)

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(string(stderr)).To(BeEmpty())
		Expect(string(stdout)).To(ContainSubstring("Repository Baseline"))
	})

	It("scans a Go module that embeds a file past the limit, because the path is still there", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", "module example.com/embedder\n\ngo 1.22\n")
		commitFile(repo, "main.go", "package main\n\nimport _ \"embed\"\n\n//go:embed big.bin\nvar big []byte\n\nfunc main() { _ = big }\n")
		commitBigBlob(repo, "big.bin", bigFile)

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--build-target", ".")

		Expect(exitCode).To(Equal(0), "go list must still find the file the pattern names; stderr: %s", stderr)
		Expect(string(stdout)).To(ContainSubstring("Repository Baseline"))
	})

	It("names a source file past the limit as too large, with its path, and goes on", func() {
		repo := commitScannableGoRepo()
		commitBigBlob(repo, "generated.go", bigFile)

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(string(stdout)).To(ContainSubstring("path: generated.go, kind: file_too_large"))
		Expect(string(stdout)).To(ContainSubstring("tracked files discovered: 2"), "the other file is still read")
	})
})
