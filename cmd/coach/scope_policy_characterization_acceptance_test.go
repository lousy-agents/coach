package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These specs pin behavior that already holds on v0.12.0 and must keep holding when scope.policy is added, so
// adding the field cannot change the text output or the flag surface. The specs for the scope.policy field itself are
// in scope_policy_disclosure_acceptance_test.go.
var _ = Describe("coach codesignal keeps its v0.12.0 text and flag surface when scope.policy is added (#168 A1)", func() {
	policyFixtures := []struct {
		label    string
		policies []scopePolicyFile
	}{
		{label: "no_policy"},
		{label: "committed_policy", policies: []scopePolicyFile{{coachConfigPath, singleRootPolicyJSON}}},
	}
	for _, mode := range scopePolicyModes {
		for _, fixture := range policyFixtures {
			It("#168 item 10: the text of a clean "+fixture.label+" run with "+mode.name+" is byte-identical to v0.12.0's", func() {
				repo, base := scopePolicyRepo(fixture.policies...)
				golden, err := os.ReadFile(filepath.Join("testdata", "v0_12_0_text", fixture.label+"_"+mode.golden+".txt"))
				Expect(err).NotTo(HaveOccurred())

				stdout, stderr, exitCode := mode.run(repo, base)

				Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
				Expect(scopePolicyNormalized(string(stdout), scopePolicyHead(repo), base)).To(Equal(string(golden)))
			})
		}
	}

	rejectedFlags := []struct {
		args     []string
		rejected string
	}{
		{args: []string{"--fail-on-policy-differs"}, rejected: "fail-on-policy-differs"},
		{args: []string{"--policy-source", "committed"}, rejected: "policy-source"},
		{args: []string{"--policy-source=disk"}, rejected: "policy-source"},
	}
	for _, mode := range scopePolicyModes {
		for _, flag := range rejectedFlags {
			It("#168 item 12: "+flag.args[0]+" with "+mode.name+" is a usage error that exits 2", func() {
				repo, base := scopePolicyRepo(scopePolicyFile{coachConfigPath, singleRootPolicyJSON})

				stdout, stderr, exitCode := mode.run(repo, base, flag.args...)

				Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
				Expect(stdout).To(BeEmpty(), "a usage error writes no report")
				Expect(string(stderr)).To(ContainSubstring("flag provided but not defined: -" + flag.rejected))
			})
		}
	}
})
