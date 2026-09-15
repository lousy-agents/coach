package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A normal --baseline scan never passes --project-config explicitly in this
// suite (every existing TypeScript project-analysis spec opts in via
// --project-config, a path this preflight leaves untouched); these specs
// exercise the previously-unhandled default path where Coach must decide,
// on its own, whether guided setup or policy authoring would be needed
// before adopting project-config-driven TypeScript project analysis. Only
// --baseline is gated: a --base diff scan's --project-language flag stays
// the documented silent no-op (docs/cli-codesignal.md), since
// prepareProjectAnalysis never consumes a policy or compiler without an
// explicit --project-config regardless of --base/--baseline.
var _ = Describe("coach codesignal --baseline --project-language typescript (normal scan, no --check-project/--prepare-compiler/--suggest-project-config)", func() {
	When("no controlling terminal is available and the TypeScript project has no committed policy", func() {
		It("performs no setup mutation, leaves stdout empty, prints remediation commands to stderr, and exits 2 (AC-SET-9, JSON format)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			// Deliberately no project.json: checks.policy fails
			// (policy_missing), the only gap this fixture carries.

			path := pathWithStubNode("v24.9.9")
			requireStubNodeVersion(path, "v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--project-language", "typescript", "--format", "json")

			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")
			transcript := string(stderr)
			Expect(transcript).To(ContainSubstring("no controlling terminal"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("coach codesignal --baseline --suggest-project-config --project-language typescript"), "transcript: %s", transcript)

			_, statErr := os.Stat(filepath.Join(repo, "project.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "no policy file must be written when preparation cannot proceed")
		})

		It("prints the same remediation in the default text format", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--project-language", "typescript")

			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")
			transcript := string(stderr)
			Expect(transcript).To(ContainSubstring("no controlling terminal"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("coach codesignal --baseline --suggest-project-config --project-language typescript"), "transcript: %s", transcript)
		})

		It("does not block a --base diff scan with the same missing policy, since --project-language alone is a silent no-op there (AC-2 fix: the gate must only fire where the scan genuinely consumes a policy or compiler)", func() {
			repo := newTempGitRepo()
			baseSHA := commitFile(repo, "README.md", "seed\n")
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			// Same missing-policy shape as above, but reached via --base:
			// prepareProjectAnalysis is a no-op here too, so this must
			// complete as an ordinary schema-1 scan rather than block.

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--base", baseSHA, "--project-language", "typescript", "--format", "json")

			Expect(string(stderr)).NotTo(ContainSubstring("no controlling terminal"), "stderr: %s", stderr)
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).NotTo(BeEmpty(), "an unblocked scan must still produce a report on stdout")

			var report struct {
				SchemaVersion string `json:"schema_version"`
			}
			Expect(json.Unmarshal(stdout, &report)).To(Succeed(), "stdout must be a parseable JSON report: %s", stdout)
			Expect(report.SchemaVersion).To(Equal("1"), "no --project-config means the unchanged schema-1 path, per docs/cli-codesignal.md")
		})
	})

	When("no controlling terminal is available and the TypeScript project has a committed policy but no locatable compiler", func() {
		It("performs no mise mutation, leaves stdout empty, prints the compiler-setup remediation command to stderr, and exits 2 (AC-SET-9, JSON format)", func() {
			repo := noSupportedCompilerRepo()
			path, miseDir := pathWithStatefulStubNodeAndMise("v24.9.9", "7.0.2")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--project-language", "typescript", "--format", "json")

			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty(), "stdout must stay reserved for the final report; none was produced")
			transcript := string(stderr)
			Expect(transcript).To(ContainSubstring("no controlling terminal"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("coach codesignal --baseline --prepare-compiler --project-language typescript"), "transcript: %s", transcript)

			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "a no-TTY scan must never invoke `mise install`")
		})
	})

	When("HEAD is fully ready (a committed policy, a locatable compiler, and a supported Node major)", func() {
		It("does not block on the missing controlling terminal, and completes the scan (AC-18's converse)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--project-language", "typescript", "--format", "json")

			Expect(string(stderr)).NotTo(ContainSubstring("no controlling terminal"), "stderr: %s", stderr)
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).NotTo(BeEmpty(), "a completed scan must produce a report on stdout")

			var report struct {
				SchemaVersion string `json:"schema_version"`
			}
			Expect(json.Unmarshal(stdout, &report)).To(Succeed(), "stdout must be a parseable JSON report: %s", stdout)
			Expect(report.SchemaVersion).To(Equal("1"), "no --project-config means the unchanged schema-1 path even once the readiness gate clears")
		})
	})

	When("readiness shows a gap this preflight cannot remediate interactively", func() {
		It("does not block on the missing controlling terminal when the only gap is an unsupported Node major (no interactive flow of its own)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v21.1.0")
			requireStubNodeVersion(path, "v21.1.0")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--project-language", "typescript", "--format", "json")

			Expect(string(stderr)).NotTo(ContainSubstring("no controlling terminal"), "stderr: %s", stderr)
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).NotTo(BeEmpty(), "a completed scan must produce a report on stdout")
		})
	})
})
