package main

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

var _ = Describe("the scan's compiler-setup offer when the policy blocks it", func() {
	DescribeTable("names the remedy readiness named for the policy",
		func(remedy projectreadiness.PolicyRemedy, want, unwanted []string) {
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()
			wrapped := &tssetup.CompilerUnresolvedErrorWithReadiness{
				CompilerUnresolvedError: &tstoolchain.CompilerUnresolvedError{Code: projectreadiness.GapTypescriptCompilerMissing, Policy: policyref.Named("pol.json")},
				Readiness: &projectreadiness.Result{
					Checks: projectreadiness.Checks{Policy: projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing, Remedy: remedy}},
				},
				Revision: "abc123",
			}

			exitCode := runScanCompilerSetupOffer(GinkgoT().TempDir(), codesignalFlags{}, os.Stdin, stdoutFile, stderrFile, wrapped, newScanOfferBudget(), defaultScanPorts())

			transcript := readStderr()
			Expect(exitCode).To(Equal(2), "stderr: %s", transcript)
			Expect(readStdout()).To(BeEmpty())
			Expect(transcript).To(ContainSubstring("a reviewed, committed policy is required before compiler setup (policy_missing)"))
			for _, text := range want {
				Expect(transcript).To(ContainSubstring(text), "stderr: %s", transcript)
			}
			for _, text := range unwanted {
				Expect(transcript).NotTo(ContainSubstring(text), "stderr: %s", transcript)
			}
		},
		Entry("a committed policy is restored", projectreadiness.RemedyRestore,
			[]string{"restore_policy", "git restore --source=abc123", "':(top,literal)pol.json'"}, []string{"author_policy", "guided policy authoring"}),
		Entry("a policy that was never committed is authored", projectreadiness.RemedyAuthor,
			[]string{"author_policy", "guided policy authoring"}, []string{"restore_policy", "git restore"}),
		Entry("a policy a new file cannot fix names neither", projectreadiness.RemedyNone,
			[]string{"resolve the policy path first"}, []string{"restore_policy", "author_policy", "git restore"}),
	)
})
