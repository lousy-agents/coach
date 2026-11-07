//go:build linux

package main

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A committed policy over the size budget is fixed by a new document as one that is not valid is, so a TypeScript scan
// on a terminal opens guided authoring for it, prints the candidate, and says which file to save it over. The
// closing instruction must not say to create the file: it exists, and authoring would refuse a target that does.
var _ = Describe("coach codesignal (TypeScript, controlling terminal): a committed policy over the size budget", func() {
	DescribeTable("opens guided authoring, prints the candidate and names the file to replace",
		func(policyPath string, scanArgs ...string) {
			repo := commitStubToolchainTypeScriptRepo(policyPath, bigPolicy())
			args := append([]string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json"}, scanArgs...)

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), discoveredAuthoringSessionAnswers, args...)

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			var candidate map[string]any
			Expect(json.Unmarshal(stdout, &candidate)).To(Succeed(), "stdout must be the candidate policy document; stdout: %s", stdout)
			Expect(candidate).To(HaveKey("roots"), "stdout must be the candidate, not a report")
			Expect(string(stderr)).To(ContainSubstring(fmt.Sprintf("save it as %q replacing the current file", policyPath)), "stderr: %s", stderr)
		},
		Entry("found at the canonical path", ".lousy-agents/coach.json"),
		Entry("named with --project-config", "pol.json", "--project-config", "pol.json"),
		Entry("named with --project-config at the canonical path", ".lousy-agents/coach.json", "--project-config", ".lousy-agents/coach.json"),
	)
})
