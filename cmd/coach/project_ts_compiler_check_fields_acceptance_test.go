package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: compiler origin aggregation (SA-280-043/044)", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
	})

	DescribeTable("checks.compiler carries only the frozen fields, whatever the outcome",
		func(fixture func(repo string), miseVersion string, wantCode string) {
			commitFile(repo, "project.json", singleRootPolicyJSON)
			fixture(repo)

			path := pathWithStubNode("v24.9.9")
			if miseVersion != "" {
				path, _ = pathWithStubMiseDefaultTool("v24.9.9", miseVersion)
			}

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var raw struct {
				Checks struct {
					Compiler map[string]json.RawMessage `json:"compiler"`
				} `json:"checks"`
			}
			Expect(json.Unmarshal(stdout, &raw)).To(Succeed(), "stdout: %s", stdout)

			var code string
			if encoded, ok := raw.Checks.Compiler["code"]; ok {
				Expect(json.Unmarshal(encoded, &code)).To(Succeed())
			}
			Expect(code).To(Equal(wantCode), "fixture must reach the intended outcome for this guard to mean anything, got %v", raw.Checks.Compiler)

			frozen := map[string]bool{
				"state": true, "code": true, "version": true, "expected_version": true,
				"found_version": true, "supported_versions": true, "root_findings": true, "detail": true,
			}
			for field := range raw.Checks.Compiler {
				Expect(frozen).To(HaveKey(field), "checks.compiler carries only the frozen fields, got %v", raw.Checks.Compiler)
			}
		},
		Entry("pass from mise with a declaration warning", func(repo string) {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"5.9.3"}}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
		}, "7.0.2", "compiler_declaration_mismatch"),
		Entry("typescript_version_mismatch", func(repo string) {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "5.9.3")
		}, "", "typescript_version_mismatch"),
		Entry("typescript_compiler_missing, where the origin classes are populated", func(repo string) {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
		}, "", "typescript_compiler_missing"),
		Entry("typescript_version_conflict", func(repo string) {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","dependencies":{"typescript":"7.0.2"},"devDependencies":{"typescript":"5.4.0"}}`+"\n")
		}, "", "typescript_version_conflict"),
	)
})
