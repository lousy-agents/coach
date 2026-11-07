package main

import (
	"encoding/json"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The Go Coach runs on a committed tree is the one on PATH, whatever the caller's GOTOOLCHAIN says, because a go.mod there
// could name a toolchain for go to download and run. The CLI contract says what follows, and these specs hold the
// contract to what the scan does: a repository that needs a newer Go than the installed one fails, and the way out is the
// one the contract gives (install that Go, or put it first on PATH), never a download.
const goNewerThanAnyInstalled = "module example.com/m\n\ngo 1.99\n"

var _ = Describe("a Go repository whose go.mod requires a newer Go than the go on PATH", func() {
	var repo string
	var callerEnv []string

	BeforeEach(func() {
		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goNewerThanAnyInstalled)
		callerEnv = append(os.Environ(), "GOTOOLCHAIN=auto")
	})

	It("fails --build-target with exit 1 and go's own message, even when the caller sets GOTOOLCHAIN", func() {
		commitFile(repo, "main.go", minimalGoSource)

		_, stderr, exitCode := runCoachBinary(commandPath, repo, callerEnv, "codesignal", "--baseline", "--build-target", "./...")

		Expect(exitCode).To(Equal(1), "stderr: %s", stderr)
		Expect(string(stderr)).To(ContainSubstring("go.mod requires go >= 1.99"))
		Expect(string(stderr)).To(ContainSubstring("GOTOOLCHAIN=local"), "the message names the setting that go ran with, not the caller's")
	})

	It("reports the call graph and the required_layer search of a Go project as incomplete, with go's message", func() {
		commitFile(repo, ".lousy-agents/coach.json", goLayerBypassPolicyConfigJSON)
		commitFile(repo, "pkg/handlers/h.go", "package handlers\n\nfunc H() {}\n")
		commitFile(repo, "pkg/service/s.go", "package service\n\nfunc S() {}\n")
		commitFile(repo, "pkg/db/d.go", "package db\n\nfunc D() {}\n")

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, callerEnv, "codesignal", "--baseline", "--project-language", "go", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		var report struct {
			ProjectCoverage struct {
				Complete    bool `json:"complete"`
				Diagnostics []struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"diagnostics"`
			} `json:"project_coverage"`
		}
		Expect(json.Unmarshal(stdout, &report)).To(Succeed(), "stdout: %s", stdout)
		Expect(report.ProjectCoverage.Complete).To(BeFalse())
		Expect(report.ProjectCoverage.Diagnostics).To(ContainElement(SatisfyAll(
			HaveField("Code", "project_callgraph_build_failed"),
			HaveField("Message", ContainSubstring("go.mod requires go >= 1.99")),
		)))
	})

	It("is told by the CLI contract what happens and the way out", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("A repository whose `go.mod` requires a newer Go than the `go` on `PATH` therefore fails, even when the caller sets `GOTOOLCHAIN`"))
		Expect(text).To(ContainSubstring("`--build-target` exits 1 with go's own message (`go.mod requires go >= X (running go Y; GOTOOLCHAIN=local)`)"))
		Expect(text).To(ContainSubstring("`project_coverage.complete` is false, with the diagnostic `project_callgraph_build_failed` carrying the same message"))
		Expect(text).To(ContainSubstring("install the Go version the `go.mod` names, or put it first on `PATH`; Coach does not download it"))
	})
})
