package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: a committed explicit policy whose parent directory has been replaced by a file", func() {
	const (
		blockedPolicy = "policies/p.json"
		blockerNotes  = "my notes"
	)
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, blockedPolicy, singleRootPolicyJSON)
		Expect(os.RemoveAll(filepath.Join(repo, "policies"))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, "policies"), []byte(blockerNotes), 0o644)).To(Succeed())
	})

	It("names the cause and the blocking path, and offers neither a restore that would overwrite the file nor a new policy", func() {
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", blockedPolicy, "--format=json")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(stdout).To(BeEmpty())
		Expect(string(stderr)).To(ContainSubstring("project_config_missing"))
		Expect(string(stderr)).To(ContainSubstring("a parent path component is not a directory"))
		Expect(string(stderr)).To(ContainSubstring(`"policies"`), "the blocking component is named:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("move or remove it"))
		Expect(string(stderr)).To(ContainSubstring("committed at the analyzed revision"), "the message says why the path matters:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("after moving the file, restore the policy from git"), "the message says what to do next:\n%s", stderr)
		Expect(strings.TrimSpace(string(stderr))).To(HaveSuffix("."), "the message is a finished sentence:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("git restore"), "git restore would delete the blocking file:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "stderr:\n%s", stderr)

		kept, err := os.ReadFile(filepath.Join(repo, "policies"))
		Expect(err).NotTo(HaveOccurred(), "the blocking file is still a regular file")
		Expect(string(kept)).To(Equal(blockerNotes))
	})

	It("still lists the other failing gaps beside the cause for a TypeScript scan", func() {
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
		Expect(os.WriteFile(filepath.Join(repo, "policies"), []byte(blockerNotes), 0o644)).To(Succeed())

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), "codesignal", "--baseline", "--project-config", blockedPolicy, "--project-language", "typescript", "--format=json")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("a parent path component is not a directory"))
		Expect(string(stderr)).To(ContainSubstring("typescript_compiler_missing: also failing, run "), "the cause must not hide the compiler gap:\n%s", stderr)
	})

	It("names neither restore_policy nor author_policy as the next action of --check-project", func() {
		doc := expectCheckProject(repo, "--project-config", blockedPolicy)

		Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
		Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
		Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
	})
})
