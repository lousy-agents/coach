//go:build unix

package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// go list reads the source snapshot, so a Go source past the limit on one file of the snapshot (32 MiB) has to
// be in it in full. It was written as an empty file, and go list failed the scan with a parse error of its own,
// "expected 'package', found 'EOF'", where v0.11.0 went on and the analyzer named the file too large.
var _ = Describe("coach codesignal --build-target: a committed Go source past the limit on one file", func() {
	It("names the source too large, and does not tell go list that it is empty", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", "module example.com/m\n\ngo 1.22\n")
		commitFile(repo, "app/main.go", "package main\n\nimport \"example.com/m/dom\"\n\nfunc main() { dom.Run() }\n")
		commitFile(repo, "dom/run.go", "package dom\n\nfunc Run() {}\n")
		commitBigGoSource(repo, "dom/big.go", "dom", bigChangedFile)

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--build-target", "./app")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(string(stdout)).To(ContainSubstring("path: dom/big.go, kind: file_too_large"), "stdout: %s", stdout)
		Expect(string(stdout) + string(stderr)).NotTo(ContainSubstring("found 'EOF'"))
	})
})
