package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// --project-config takes a path from the repository root, whatever directory the scan starts in. A path
// spelled from a subdirectory climbs out of the root and is refused; the message says how to spell it, and
// the scan does not suggest authoring, which would write a policy somewhere else and leave the path wrong.
var _ = Describe("coach codesignal: a --project-config path spelled from a subdirectory", func() {
	It("is refused with the way to spell it and no authoring suggestion", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, "policies/p.json", singleRootPolicyJSON)
		subdirectory := filepath.Join(repo, "sub", "deep")
		Expect(os.MkdirAll(subdirectory, 0o755)).To(Succeed())

		stdout, stderr, exitCode := runCoachBinary(commandPath, subdirectory, nil, "codesignal", "--baseline", "--project-config", "../../policies/p.json", "--format=json")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("from the repository root"), "stderr:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("--suggest-project-config"), "authoring does not fix a path:\n%s", stderr)
	})
})
