package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

// --prepare-compiler computes the same readiness as --check-project, and reached the same failure to read the
// policy's blob in a partial clone, but printed git's bare "object ... is not in the local object store": no
// word that Coach does not fetch, and no command that does. The flow is entered below its check for a terminal,
// which a spec has none of, and the failure is worded as --check-project words it.
var _ = Describe("coach codesignal --prepare-compiler: a policy blob a partial clone lacks is explained", func() {
	It("says that Coach does not fetch objects, and names the command that fetches the policy", func() {
		skipUnlessGitCanRefuseLazyFetch()
		clone := commitPartialClone(func(bare string) string { return "file://" + bare })
		// the policy is on disk, so the check goes on to read its blob from the revision
		Expect(os.MkdirAll(filepath.Join(clone, "cfg"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(clone, "cfg", "p.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
		devNull, err := os.Open(os.DevNull)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(devNull.Close)
		transcript, err := os.Create(filepath.Join(GinkgoT().TempDir(), "stderr"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(transcript.Close)

		exitCode := prepareCompilerMiseTypeScript(clone, devNull, devNull, transcript, policyref.Named("cfg/p.json"))

		printed, readErr := os.ReadFile(transcript.Name())
		Expect(readErr).NotTo(HaveOccurred())
		Expect(exitCode).To(Equal(3), "stderr: %s", printed)
		Expect(string(printed)).To(ContainSubstring("could not compute project readiness"), "stderr:\n%s", printed)
		Expect(string(printed)).To(ContainSubstring(`"cfg/p.json" could not be read at revision`), "stderr:\n%s", printed)
		Expect(string(printed)).To(ContainSubstring("its content is not in this partial clone, and Coach does not fetch objects"), "stderr:\n%s", printed)
		Expect(string(printed)).To(ContainSubstring("git cat-file -e"), "the way out is a command:\n%s", printed)
		Expect(string(printed)).NotTo(ContainSubstring("is not in the local object store"), "git's bare words tell the user nothing:\n%s", printed)
	})
})
