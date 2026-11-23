package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

// A policy that is on disk is judged from the file, so a partial clone that lacks the blob of its committed copy does not
// stop the setup: the lack is said as a disclosure, and no command to fetch the policy is asked of the user.
var _ = Describe("coach codesignal --prepare-compiler: a policy on disk whose blob a partial clone lacks", func() {
	It("does not stop at the committed copy, and says the committed copy could not be read", func() {
		skipUnlessGitCanRefuseLazyFetch()
		clone := commitPartialClone(func(bare string) string { return "file://" + bare })
		Expect(os.MkdirAll(filepath.Join(clone, "cfg"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(clone, "cfg", "p.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
		run := prepareCompilerAnswering(clone, policyref.Named("cfg/p.json"), "mise_project\ndecline\n")
		printed := run.transcript

		Expect(string(printed)).NotTo(ContainSubstring(`"cfg/p.json" could not be read at revision`), "stderr:\n%s", printed)
		Expect(string(printed)).NotTo(ContainSubstring("git cat-file -e"), "no fetch is asked of the user for a policy the disk holds:\n%s", printed)
		Expect(string(printed)).To(ContainSubstring("project_policy_head_unreadable"), "stderr:\n%s", printed)
	})
})
