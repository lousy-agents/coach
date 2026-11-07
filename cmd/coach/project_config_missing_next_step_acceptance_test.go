package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const suggestCommand = "--suggest-project-config"

var _ = Describe("coach codesignal: the next step offered after an explicit --project-config is missing", func() {
	runScan := func(repo string, language string, policy string) (stdout, stderr []byte) {
		args := []string{"codesignal", "--baseline", "--project-config", policy, "--format=json"}
		if language != "" {
			args = append(args, "--project-language", language)
		}
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), args...)
		ExpectWithOffset(1, exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		ExpectWithOffset(1, string(stderr)).To(ContainSubstring("project_config_missing"))
		return stdout, stderr
	}

	Describe("a policy that is committed at the analyzed revision", func() {
		var repo string

		BeforeEach(func() {
			repo = commitScannableGoRepo()
			commitFile(repo, "pol.json", singleRootPolicyJSON)
			Expect(os.Remove(filepath.Join(repo, "pol.json"))).To(Succeed())
		})

		It("offers only the restore command, not a new policy to author", func() {
			_, stderr := runScan(repo, "", "pol.json")

			Expect(string(stderr)).To(ContainSubstring("restore it with git restore"))
			Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "a reviewed policy exists; authoring one would replace it:\n%s", stderr)
		})

		It("offers only the restore command for a TypeScript scan too", func() {
			_, stderr := runScan(repo, "typescript", "pol.json")

			Expect(string(stderr)).To(ContainSubstring("restore it with git restore"))
			Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "stderr:\n%s", stderr)
		})

		It("lists restore_policy, not author_policy, as the next action of --check-project", func() {
			doc := expectCheckProject(repo, "--project-config", "pol.json")

			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
			Expect(nextActionKinds(doc)).To(ContainElement("restore_policy"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
			Expect(nextActionsNamed(doc, "restore_policy")).To(HaveEach(HaveField("Executable", BeFalse())), "coach prints the restore command; it never runs it")
		})

		It("still lists the other failing gaps beside the restore command", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")

			_, stderr := runScan(repo, "typescript", "pol.json")

			Expect(string(stderr)).To(ContainSubstring("restore it with git restore"))
			Expect(string(stderr)).To(ContainSubstring("typescript_compiler_missing: also failing, run "), "the restore must not hide the compiler gap:\n%s", stderr)
		})
	})

	Describe("a policy that was never committed", func() {
		It("still offers authoring", func() {
			repo := commitScannableGoRepo()

			_, stderr := runScan(repo, "", "pol.json")

			Expect(string(stderr)).To(ContainSubstring(suggestCommand))
			Expect(string(stderr)).NotTo(ContainSubstring("restore it"))
		})

		It("lists author_policy as the next action of --check-project", func() {
			repo := commitScannableGoRepo()

			doc := expectCheckProject(repo, "--project-config", "pol.json")

			Expect(nextActionKinds(doc)).To(ContainElement("author_policy"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
			Expect(nextActionsNamed(doc, "author_policy")).To(HaveEach(HaveField("Executable", BeFalse())), "authoring needs a human; coach never runs it unprompted")
		})
	})

	Describe("a path that runs through a file where a directory would be", func() {
		It("names the cause and offers no authoring, since a policy cannot be saved there", func() {
			repo := commitScannableGoRepo()
			Expect(os.WriteFile(filepath.Join(repo, "notes"), []byte("untracked\n"), 0o644)).To(Succeed())

			_, stderr := runScan(repo, "", "notes/p.json")

			Expect(string(stderr)).To(ContainSubstring("a parent path component is not a directory"))
			Expect(string(stderr)).To(ContainSubstring(`"notes"`), "the blocking component is named:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("move or remove it"), "a file nothing commits can be moved:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "stderr:\n%s", stderr)
		})

		It("does not advise removing a file the analyzed revision commits, which is where the path is wrong", func() {
			repo := commitScannableGoRepo()

			_, stderr := runScan(repo, "", "a.go/p.json")

			Expect(string(stderr)).To(ContainSubstring("a parent path component is not a directory"))
			Expect(string(stderr)).To(ContainSubstring(`"a.go"`), "the blocking component is named:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("a file the analyzed revision commits, so the path is probably wrong"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("move or remove it"), "a committed file is not advised away:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "stderr:\n%s", stderr)
		})

		It("offers no authoring for a TypeScript scan either", func() {
			repo := commitScannableGoRepo()

			_, stderr := runScan(repo, "typescript", "a.go/p.json")

			Expect(string(stderr)).To(ContainSubstring("a parent path component is not a directory"))
			Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "stderr:\n%s", stderr)
		})
	})
})
