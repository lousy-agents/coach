package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal (real scan): the readiness lines printed beside a refused explicit policy", func() {
	When("the committed explicit policy names a root that has no package.json and the file was removed from disk", func() {
		It("come from the working tree's verdict, so the committed copy the scan refused names no gap of its own", func() {
			repo := newTempGitRepo()
			// A top-level package.json would satisfy the shape check whatever the roots say, hiding which policy the line came from.
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "cfg/p.json", `{"schema_version":"1","roots":["web"]}`+"\n")
			Expect(os.RemoveAll(filepath.Join(repo, "cfg"))).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, pathWithStubNode("v24.9.9"), "--project-config", "cfg/p.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring("project_config_missing"), "the scan must refuse the policy for the reason it is gone from disk; stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("unsupported_repository_shape"), "this gap is worked out from roots in the committed policy the scan just refused, and the --check-project command the line names does not report it; stderr: %s", stderr)
		})
	})
})
