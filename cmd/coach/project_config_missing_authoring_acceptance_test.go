//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal (TypeScript, controlling terminal): an explicit --project-config that does not exist on disk", func() {
	DescribeTable("offers guided authoring instead of the scan, prints the candidate on stdout, names the path and project_config_missing on stderr, and still exits 2",
		func(path string, prepare func(repo string)) {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			prepare(repo)

			stubPath := pathWithStubNode("v24.9.9")
			answers := "1\n\n\n\napprove\n"

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(stubPath), answers,
				"codesignal", "--baseline", "--project-config", path, "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)

			var candidate map[string]any
			Expect(json.Unmarshal(stdout, &candidate)).To(Succeed(), "stdout must be the candidate policy document; stdout: %s", stdout)
			Expect(candidate).To(HaveKey("roots"), "stdout must be the candidate, not a report")
			Expect(candidate).NotTo(HaveKey("scope"))
			Expect(candidate).NotTo(HaveKey("signals"))

			Expect(string(stderr)).To(ContainSubstring("project_config_missing"), "stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring(`"`+path+`"`), "stderr must name the path: %s", stderr)
			missingIndex := strings.Index(string(stderr), "project_config_missing")
			authoringIndex := strings.Index(string(stderr), "Discovered TypeScript roots")
			Expect(authoringIndex).To(BeNumerically(">", missingIndex), "the missing-policy message must come before the guided session opens; stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring(fmt.Sprintf("save it to %q at the repository root", path)), "the closing instruction says where the candidate is saved; stderr: %s", stderr)
		},
		Entry("the file was never created", "project.json", func(string) {}),
	)

	DescribeTable("does not open guided authoring when a new policy would not fix it",
		func(path string, prepare func(repo string), want ...string) {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			prepare(repo)

			stubPath := pathWithStubNode("v24.9.9")
			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(stubPath), "",
				"codesignal", "--baseline", "--project-config", path, "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "no candidate and no report")
			Expect(string(stderr)).To(ContainSubstring("project_config_missing"))
			for _, text := range want {
				Expect(string(stderr)).To(ContainSubstring(text))
			}
			Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"))
			Expect(string(stderr)).NotTo(ContainSubstring("--suggest-project-config"))
		},
		Entry("the file is committed at HEAD and deleted on disk: it is restored, not written again", "project.json", func(repo string) {
			commitFile(repo, "project.json", singleRootPolicyJSON)
			Expect(os.Remove(filepath.Join(repo, "project.json"))).To(Succeed())
		}, "restore it with git restore"),
		Entry("the path runs through a regular file: nothing can be saved there", "package.json/x.json", func(string) {}, "a parent path component is not a directory"),
	)

	It("does not open guided authoring for a dangling symbolic link, which is refused as invalid", func() {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
		Expect(os.Symlink("/nonexistent/x.json", filepath.Join(repo, "dang.json"))).To(Succeed())

		stubPath := pathWithStubNode("v24.9.9")
		stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(stubPath), "",
			"codesignal", "--baseline", "--project-config", "dang.json", "--project-language", "typescript", "--format=json")

		Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
		Expect(stdout).To(BeEmpty(), "no candidate and no report")
		Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
		Expect(string(stderr)).To(ContainSubstring("symbolic link"))
		Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"))
	})

	DescribeTable("does not open guided authoring when prompts are turned off, even with a controlling terminal attached",
		func(extraArgs []string, extraEnv []string) {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			env := append(append([]string{}, stubToolchainEnv(pathWithStubNode("v24.9.9"))...), extraEnv...)
			args := append([]string{"codesignal", "--baseline", "--project-config", "project.json", "--project-language", "typescript", "--format=json"}, extraArgs...)
			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, env, "1\n\n\n\napprove\n", args...)

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "no candidate and no report")
			Expect(string(stderr)).To(ContainSubstring("project_config_missing"))
			Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"))
		},
		Entry("the --no-interactive flag", []string{"--no-interactive"}, nil),
		Entry("a non-empty CI environment variable", nil, []string{"CI=1"}),
	)
})
