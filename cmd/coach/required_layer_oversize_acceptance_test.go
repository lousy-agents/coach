//go:build unix

package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const requiredLayerPolicy = `{"schema_version":"1","roots":["."],"layers":[{"name":"dom","prefixes":["dom"]},{"name":"app","prefixes":["app"]}],"forbidden_imports":[],"required_layer":"app"}` + "\n"

// docs/cli-codesignal.md says where a file past the limit on one file still ends a scan: a Go project whose
// policy sets required_layer copies every file of the revision to disk to build its call graph, through the
// same `git show` that holds a file to 32 MiB. v0.11.0 ended the same way, for a file of any type. The
// sentence is true as long as this spec passes; a change that lets such a scan go on is a change to the docs.
var _ = Describe("coach codesignal --baseline --project-language go: a file past the limit on one file when the policy sets required_layer", func() {
	It("ends the scan with exit 1 and names the file and the budget, as the documentation says", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", "module example.com/m\n\ngo 1.22\n")
		commitFile(repo, "app/app.go", "package app\n\nfunc Run() int { return 1 }\n")
		commitFile(repo, "dom/d.go", "package dom\n\nimport \"example.com/m/app\"\n\nfunc D() int { return app.Run() }\n")
		commitFile(repo, ".lousy-agents/coach.json", requiredLayerPolicy)
		commitBigBlob(repo, "assets/blob.bin", bigChangedFile)

		_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-language", "go")

		Expect(exitCode).To(Equal(1), "stderr: %s", stderr)
		Expect(string(stderr)).To(ContainSubstring("assets/blob.bin"))
		Expect(string(stderr)).To(ContainSubstring("git stdout exceeded 33554432-byte budget"))
	})
})
