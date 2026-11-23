package main

import (
	"bytes"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

var _ = Describe("a policy path with terminal control characters in a refusal printed for a policy that is gone from disk", func() {
	const (
		hostilePolicy = "cfg/p\x1b[31mRED\x07.json"
		quotedPolicy  = `"cfg/p\x1b[31mRED\a.json"`
	)

	type goneFromDisk struct {
		arrange func(repo string)
	}

	stagedEditThenDeleted := func(repo string) {
		commitFile(repo, hostilePolicy, singleRootPolicyJSON)
		writeWorktreeFile(repo, hostilePolicy, validPolicyWithLayers)
		gitIn(repo, "add", "--", hostilePolicy)
		Expect(os.Remove(filepath.Join(repo, hostilePolicy))).To(Succeed())
	}
	committedAsLinkThenRemoved := func(repo string) {
		commitFile(repo, hostilePolicy, singleRootPolicyJSON)
		typechangeFileToSymlink(repo, hostilePolicy, "elsewhere.json")
		Expect(os.Remove(filepath.Join(repo, hostilePolicy))).To(Succeed())
	}

	DescribeTable("--prepare-compiler prints the path quoted and no raw control byte",
		func(state goneFromDisk) {
			repo := typescriptRepoWithoutPolicy()
			state.arrange(repo)

			run := prepareCompilerAnswering(repo, policyref.Named(hostilePolicy), "")

			Expect(run.exitCode).To(Equal(2), "transcript:\n%q", run.transcript)
			Expect(run.transcript).To(ContainSubstring(quotedPolicy), "transcript:\n%q", run.transcript)
			Expect(bytes.ContainsAny([]byte(run.transcript), "\x1b\x07")).To(BeFalse(), "a control byte reached the terminal:\n%q", run.transcript)
		},
		Entry("the index holds a staged edit and the file was removed", goneFromDisk{stagedEditThenDeleted}),
		Entry("the revision commits a link there and the link was removed", goneFromDisk{committedAsLinkThenRemoved}),
	)

	DescribeTable("--check-project prints no raw control byte in the text output",
		func(state goneFromDisk) {
			repo := typescriptRepoWithoutPolicy()
			state.arrange(repo)

			stdout, stderr, _ := runCoachCheckProjectEnv(repo, pathWithStubNode("v24.9.9"), "--baseline", "--check-project", "--project-language", "typescript", "--project-config", hostilePolicy)

			output := append(append([]byte{}, stdout...), stderr...)
			Expect(bytes.ContainsAny(output, "\x1b\x07")).To(BeFalse(), "a control byte reached the terminal:\n%q", output)
		},
		Entry("the index holds a staged edit and the file was removed", goneFromDisk{stagedEditThenDeleted}),
		Entry("the revision commits a link there and the link was removed", goneFromDisk{committedAsLinkThenRemoved}),
	)
})
