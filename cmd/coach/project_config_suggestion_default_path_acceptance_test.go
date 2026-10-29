package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach suggest project config default path", func() {
	When("coach codesignal --baseline --suggest-project-config is run from a subdirectory and .lousy-agents/coach.json does not exist", func() {
		It("exits 0, prints the candidate on stdout, and writes that same document to .lousy-agents/coach.json at the repository root", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/defaultpath\n\ngo 1.25\n")
			commitFile(repo, "main.go", "package main\n\nfunc main() {}\n")
			subdir := filepath.Join(repo, "nested")
			Expect(os.MkdirAll(subdir, 0o755)).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(subdir, "--suggest-project-config")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_ready"))

			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(stdout, &candidate)).To(Succeed(), "stdout: %s", stdout)
			Expect(candidate.SchemaVersion).To(Equal("1"))
			Expect(candidate.Roots).To(Equal([]string{"."}))

			written, err := os.ReadFile(filepath.Join(repo, ".lousy-agents", "coach.json"))
			Expect(err).NotTo(HaveOccurred(), "the candidate must be written at the repository root, not the invocation subdirectory")
			Expect(written).To(Equal(stdout), "the default-path file must be the same document printed on stdout")

			_, statErr := os.Stat(filepath.Join(repo, "project.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "the default destination is .lousy-agents/coach.json, not project.json")
			_, statErr = os.Stat(filepath.Join(subdir, ".lousy-agents", "coach.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "the default file must not be written under the invocation subdirectory")
		})
	})

	When("--output is set", func() {
		It("writes only that path and does not create .lousy-agents/coach.json", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/explicitoutput\n\ngo 1.25\n")
			commitFile(repo, "main.go", "package main\n\nfunc main() {}\n")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--suggest-project-config", "--output", "other.json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty(), "an explicit --output write must not also print the candidate")

			written, err := os.ReadFile(filepath.Join(repo, "other.json"))
			Expect(err).NotTo(HaveOccurred())
			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(written, &candidate)).To(Succeed(), "other.json: %s", written)
			Expect(candidate.Roots).To(Equal([]string{"."}))

			_, statErr := os.Stat(filepath.Join(repo, ".lousy-agents", "coach.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "--output must be the only file written")
		})
	})

	When(".lousy-agents/coach.json already exists", func() {
		It("exits 2 with project_config_suggestion_output_exists, leaves the file unchanged, and prints no candidate on stdout", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/defaultexists\n\ngo 1.25\n")
			commitFile(repo, "main.go", "package main\n\nfunc main() {}\n")

			original := []byte("do-not-touch\n")
			Expect(os.MkdirAll(filepath.Join(repo, ".lousy-agents"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, ".lousy-agents", "coach.json"), original, 0o644)).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--suggest-project-config")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty(), "a create-only refusal must not print the candidate")
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_exists"))

			untouched, err := os.ReadFile(filepath.Join(repo, ".lousy-agents", "coach.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(untouched).To(Equal(original), "an existing default-path target must not be overwritten or deleted")
		})
	})

	When(".lousy-agents is a symlink", func() {
		It("exits 2 with project_config_suggestion_output_invalid and does not write through the link", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/defaultsymlink\n\ngo 1.25\n")
			commitFile(repo, "main.go", "package main\n\nfunc main() {}\n")

			elsewhere, err := os.MkdirTemp("", "coach-acceptance-default-symlink-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, elsewhere)
			Expect(os.Symlink(elsewhere, filepath.Join(repo, ".lousy-agents"))).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--suggest-project-config")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty(), "a rejected symlink parent must not print the candidate")
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_invalid"))

			_, statErr := os.Stat(filepath.Join(elsewhere, "coach.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "the default write must not follow a symlinked .lousy-agents directory")
		})
	})

	When("TypeScript guided authoring is confirmed without --output", func() {
		It("writes .lousy-agents/coach.json create-only and does not print the candidate document on stdout", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			stdin := authoringStdin("1\n\n\n\napprove\n")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript"}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(0), "stderr: %s", readStderr())
			Expect(readStdout()).NotTo(ContainSubstring(`"schema_version"`), "the candidate document must not be written to stdout when the default path is used; stdout: %s", readStdout())
			Expect(readStderr()).To(ContainSubstring("Select the roots to include"))

			data, err := os.ReadFile(filepath.Join(repo, ".lousy-agents", "coach.json"))
			Expect(err).NotTo(HaveOccurred(), "confirmation without --output must write .lousy-agents/coach.json")
			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(data, &candidate)).To(Succeed(), "written file: %s", data)
			Expect(candidate.SchemaVersion).To(Equal("1"))
			Expect(candidate.Roots).To(Equal([]string{"."}))
		})
	})

	When("TypeScript guided authoring declines approval without --output", func() {
		It("exits 2 after the session and leaves both .lousy-agents and .lousy-agents/coach.json absent", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			stdin := authoringStdin("1\n\n\n\nno\n")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript"}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(2), "stderr: %s", readStderr())
			Expect(readStderr()).To(ContainSubstring("Select the roots to include"), "the decline must happen inside the guided session, not before it; stderr: %s", readStderr())
			Expect(readStderr()).To(ContainSubstring("cancelled or not approved"), "stderr: %s", readStderr())
			Expect(readStdout()).NotTo(ContainSubstring(`"schema_version"`), "a declined session must not emit a candidate; stdout: %s", readStdout())

			_, dirErr := os.Lstat(filepath.Join(repo, ".lousy-agents"))
			Expect(os.IsNotExist(dirErr)).To(BeTrue(), "declining approval must not create .lousy-agents; Lstat err = %v", dirErr)
			_, fileErr := os.Lstat(filepath.Join(repo, ".lousy-agents", "coach.json"))
			Expect(os.IsNotExist(fileErr)).To(BeTrue(), "declining approval must not create .lousy-agents/coach.json; Lstat err = %v", fileErr)
		})
	})

	When("TypeScript guided authoring omits --output and .lousy-agents/coach.json already exists", func() {
		It("exits 2 before prompting, reports that the target already exists, and leaves the original bytes unchanged", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			original := []byte("do-not-touch\n")
			Expect(os.MkdirAll(filepath.Join(repo, ".lousy-agents"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, ".lousy-agents", "coach.json"), original, 0o644)).To(Succeed())

			stdin := authoringStdin("1\n\n\n\napprove\n")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript"}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(2), "stderr: %s", readStderr())
			Expect(readStderr()).To(ContainSubstring("already exists"), "stderr: %s", readStderr())
			Expect(readStderr()).NotTo(ContainSubstring("Select the roots"), "an existing default target must be refused before the guided session prompts; stderr: %s", readStderr())
			Expect(readStdout()).NotTo(ContainSubstring("Select the roots"), "stdout: %s", readStdout())

			untouched, err := os.ReadFile(filepath.Join(repo, ".lousy-agents", "coach.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(untouched).To(Equal(original), "an existing default-path target must not be overwritten or deleted")
		})
	})

	When("TypeScript guided authoring is confirmed with --output", func() {
		It("writes only that path and does not create .lousy-agents/coach.json", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			stdin := authoringStdin("1\n\n\n\napprove\n")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript", output: "out.json", outputSet: true}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(0), "stderr: %s", readStderr())
			Expect(readStdout()).NotTo(ContainSubstring(`"schema_version"`), "stdout: %s", readStdout())

			data, err := os.ReadFile(filepath.Join(repo, "out.json"))
			Expect(err).NotTo(HaveOccurred())
			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(data, &candidate)).To(Succeed(), "out.json: %s", data)
			Expect(candidate.Roots).To(Equal([]string{"."}))

			_, statErr := os.Stat(filepath.Join(repo, ".lousy-agents", "coach.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "--output must remain the only file written")
		})
	})
})
