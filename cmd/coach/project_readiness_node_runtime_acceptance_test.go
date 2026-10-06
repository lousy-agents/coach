package main

import (
	"encoding/json"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript", func() {
	When("Node cannot be found on the child process's PATH at all", func() {
		It("reports the node check as fail/node_missing deterministically, mirrored exactly by checks.runtime, with an install_supported_runtime next action", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithoutNode()
			requireNodeUnreachable(path)

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_missing"))
			Expect(doc.Checks.Node.Kind).To(BeEmpty(), "checks.node must never mirror kind")
			Expect(doc.Checks.Node.Origin).To(BeEmpty(), "checks.node must never mirror origin")
			Expect(doc.Checks.Runtime.State).To(Equal("fail"))
			Expect(doc.Checks.Runtime.Code).To(Equal("node_missing"))
			Expect(doc.Checks.Runtime.Kind).To(Equal("node"))
			Expect(doc.Checks.Runtime.Origin).To(Equal("path"))
			Expect(doc.Checks.Node.State).To(Equal(doc.Checks.Runtime.State))
			Expect(doc.Checks.Node.Code).To(Equal(doc.Checks.Runtime.Code))
			Expect(doc.Checks.Node.Version).To(Equal(doc.Checks.Runtime.Version))
			Expect(doc.Status).To(Equal("needs_prerequisite"))

			action, _ := nextActionOfKind(doc, "install_supported_runtime")
			Expect(action.Kind).To(Equal("install_supported_runtime"))
			Expect(action.Executable).To(BeFalse())
			Expect(action.RuntimeKind).To(Equal("node"))
			Expect(action.Supported).To(Equal([]string{"24", "26"}))
			Expect(action.FoundVersion).To(BeEmpty(), "node_missing has no probed version to report")
			Expect(action.Detail).To(BeEmpty(), "install_supported_runtime carries no detail field")

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)
			Expect(text).To(ContainSubstring("runtime: fail (node_missing) kind=node origin=path"))
			Expect(text).To(ContainSubstring("install_supported_runtime (executable=false) runtime_kind=node supported=24,26"))
		})
	})

	When("the resolvable Node's major version is outside the supported set, below every member", func() {
		It("reports the node check as fail/node_unsupported with the found version, deterministically", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v22.10.0")
			requireStubNodeVersion(path, "v22.10.0")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_unsupported"))
			Expect(doc.Checks.Node.Version).To(Equal("v22.10.0"))
			Expect(doc.Checks.Runtime.State).To(Equal("fail"))
			Expect(doc.Checks.Runtime.Code).To(Equal("node_unsupported"))
			Expect(doc.Checks.Runtime.Version).To(Equal("v22.10.0"))
			Expect(doc.Checks.Runtime.Kind).To(Equal("node"))
			Expect(doc.Checks.Runtime.Origin).To(Equal("path"))
			Expect(doc.Status).To(Equal("needs_prerequisite"))

			action, _ := nextActionOfKind(doc, "install_supported_runtime")
			Expect(action.Kind).To(Equal("install_supported_runtime"))
			Expect(action.Executable).To(BeFalse())
			Expect(action.RuntimeKind).To(Equal("node"))
			Expect(action.Supported).To(Equal([]string{"24", "26"}))
			Expect(action.FoundVersion).To(Equal("v22.10.0"), "node_unsupported carries the probed version that fell outside the supported set")
			Expect(action.Detail).To(BeEmpty(), "install_supported_runtime carries no detail field")
		})
	})

	When("the resolvable Node's major version is outside the supported set, above every member", func() {
		It("reports the node check as fail/node_unsupported, proving the set membership swap fires above the set too, not merely below a retired floor", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v27.0.0")
			requireStubNodeVersion(path, "v27.0.0")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_unsupported"))
			Expect(doc.Checks.Node.Version).To(Equal("v27.0.0"))
			Expect(doc.Checks.Runtime.Code).To(Equal("node_unsupported"))
			Expect(doc.Status).To(Equal("needs_prerequisite"))
		})
	})

	When("Node resolves to the supported major 24", func() {
		It("reports the node check as pass with no code, warning, or gap, and checks.runtime carries kind/origin", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v24.9.9")
			requireStubNodeVersion(path, "v24.9.9")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("pass"))
			Expect(doc.Checks.Node.Code).To(BeEmpty())
			Expect(doc.Checks.Node.Version).To(Equal("v24.9.9"))
			Expect(doc.Checks.Node.Kind).To(BeEmpty())
			Expect(doc.Checks.Node.Origin).To(BeEmpty())
			Expect(doc.Checks.Runtime.State).To(Equal("pass"))
			Expect(doc.Checks.Runtime.Code).To(BeEmpty())
			Expect(doc.Checks.Runtime.Version).To(Equal("v24.9.9"))
			Expect(doc.Checks.Runtime.Kind).To(Equal("node"))
			Expect(doc.Checks.Runtime.Origin).To(Equal("path"))
			Expect(doc.Status).To(Equal("ready"))
			Expect(doc.Warnings).To(BeEmpty(), "a supported Node major must never emit a warning")
		})
	})

	When("Node resolves to the supported major 26", func() {
		It("reports the node check as pass with no code, warning, or gap, proving 26 is a first-class supported major rather than an untested one", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("v26.0.0")
			requireStubNodeVersion(path, "v26.0.0")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Node.State).To(Equal("pass"))
			Expect(doc.Checks.Node.Code).To(BeEmpty())
			Expect(doc.Checks.Node.Version).To(Equal("v26.0.0"))
			Expect(doc.Checks.Runtime.State).To(Equal("pass"))
			Expect(doc.Checks.Runtime.Code).To(BeEmpty())
			Expect(doc.Checks.Runtime.Version).To(Equal("v26.0.0"))
			Expect(doc.Status).To(Equal("ready"))
			Expect(doc.Warnings).To(BeEmpty(), "a supported Node major must never emit a warning, and node_untested no longer exists")

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)
			Expect(text).To(ContainSubstring("status: ready"))
			Expect(text).To(ContainSubstring("runtime: pass kind=node version=v26.0.0 origin=path"))
			Expect(text).NotTo(ContainSubstring("node_untested"))
		})
	})

	When("Node is on PATH but `node --version` hangs indefinitely", func() {
		It("still exits within a bounded wall clock, reporting node fail/node_unverifiable with a bounded stderr-free detail, and still produces the fit report", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithHangingNode()

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.SchemaVersion).NotTo(BeEmpty(), "the readiness document must still be produced on a probe failure")
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_unverifiable"))
			Expect(doc.Checks.Node.Detail).To(BeEmpty(), "checks.node must never mirror detail")
			Expect(doc.Checks.Runtime.State).To(Equal("fail"))
			Expect(doc.Checks.Runtime.Code).To(Equal("node_unverifiable"))
			Expect(doc.Checks.Runtime.Kind).To(Equal("node"))
			Expect(doc.Checks.Runtime.Origin).To(Equal("path"))
			Expect(doc.Checks.Runtime.Detail).To(Equal("node --version timed out"))
			Expect(doc.Checks.Runtime.Detail).NotTo(ContainSubstring(string(os.PathSeparator)), "the detail must not leak the stub node's host path")
			Expect(doc.Status).To(Equal("needs_prerequisite"))

			action, _ := nextActionOfKind(doc, "repair_runtime_probe")
			Expect(action.Kind).To(Equal("repair_runtime_probe"))
			Expect(action.Executable).To(BeFalse())
			Expect(action.RuntimeKind).To(Equal("node"))
			Expect(action.Supported).To(BeEmpty(), "repair_runtime_probe carries no supported field")
			Expect(action.FoundVersion).To(BeEmpty(), "repair_runtime_probe carries no found_version field")
			Expect(action.Detail).To(Equal("node --version timed out"))

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)
			Expect(text).To(ContainSubstring("runtime: fail (node_unverifiable) kind=node origin=path"))
			Expect(text).To(ContainSubstring("repair_runtime_probe (executable=false) runtime_kind=node detail=node --version timed out"))
		})
	})

	When("Node is on PATH but `node --version` exits non-zero", func() {
		It("reports node fail/node_unverifiable with a bounded path-free detail and a repair_runtime_probe next action, not node_missing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithFailingNode()

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.SchemaVersion).NotTo(BeEmpty(), "the readiness document must still be produced on a probe failure")
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_unverifiable"))
			Expect(doc.Checks.Node.Detail).To(BeEmpty(), "checks.node must never mirror detail")
			Expect(doc.Checks.Runtime.State).To(Equal("fail"))
			Expect(doc.Checks.Runtime.Code).To(Equal("node_unverifiable"))
			Expect(doc.Checks.Runtime.Kind).To(Equal("node"))
			Expect(doc.Checks.Runtime.Origin).To(Equal("path"))
			Expect(doc.Checks.Runtime.Detail).To(Equal("node --version failed: running node --version: exit status 3"))
			Expect(doc.Checks.Runtime.Detail).NotTo(ContainSubstring(string(os.PathSeparator)), "the detail must not leak the stub node's host path")
			Expect(doc.Status).To(Equal("needs_prerequisite"))

			action, _ := nextActionOfKind(doc, "repair_runtime_probe")
			Expect(action.Kind).To(Equal("repair_runtime_probe"))
			Expect(action.Executable).To(BeFalse())
			Expect(action.RuntimeKind).To(Equal("node"))
			Expect(action.Supported).To(BeEmpty(), "repair_runtime_probe carries no supported field")
			Expect(action.FoundVersion).To(BeEmpty(), "repair_runtime_probe carries no found_version field")
			Expect(action.Detail).To(Equal("node --version failed: running node --version: exit status 3"))

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)
			Expect(text).To(ContainSubstring("runtime: fail (node_unverifiable) kind=node origin=path"))
			Expect(text).To(ContainSubstring("repair_runtime_probe (executable=false) runtime_kind=node detail=node --version failed: running node --version: exit status 3"))
		})
	})

	When("Node is on PATH but the `node --version` process cannot start", func() {
		It("reports node fail/node_unverifiable with a bounded, path-free start-failure detail and a repair_runtime_probe next action", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithUnstartableNode()

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.SchemaVersion).NotTo(BeEmpty(), "the readiness document must still be produced on a probe failure")
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_unverifiable"))
			Expect(doc.Checks.Node.Detail).To(BeEmpty(), "checks.node must never mirror detail")
			Expect(doc.Checks.Runtime.State).To(Equal("fail"))
			Expect(doc.Checks.Runtime.Code).To(Equal("node_unverifiable"))
			Expect(doc.Checks.Runtime.Kind).To(Equal("node"))
			Expect(doc.Checks.Runtime.Origin).To(Equal("path"))
			Expect(doc.Checks.Runtime.Detail).To(Equal("node --version failed to start: no such file or directory"))
			Expect(doc.Checks.Runtime.Detail).NotTo(ContainSubstring(string(os.PathSeparator)), "the detail must not leak the stub node's host path")
			Expect(doc.Checks.Runtime.Detail).NotTo(ContainSubstring("fork/exec"), "the detail must not leak the raw fork/exec diagnostic")
			Expect(doc.Status).To(Equal("needs_prerequisite"))

			action, _ := nextActionOfKind(doc, "repair_runtime_probe")
			Expect(action.Kind).To(Equal("repair_runtime_probe"))
			Expect(action.Executable).To(BeFalse())
			Expect(action.RuntimeKind).To(Equal("node"))
			Expect(action.Supported).To(BeEmpty(), "repair_runtime_probe carries no supported field")
			Expect(action.FoundVersion).To(BeEmpty(), "repair_runtime_probe carries no found_version field")
			Expect(action.Detail).To(Equal("node --version failed to start: no such file or directory"))

			textStdout, textStderr, textExit := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)
			Expect(text).To(ContainSubstring("runtime: fail (node_unverifiable) kind=node origin=path"))
			Expect(text).To(ContainSubstring("repair_runtime_probe (executable=false) runtime_kind=node detail=node --version failed to start: no such file or directory"))
		})
	})

	When("Node resolves and runs but prints output that is not a parsable version string", func() {
		It("reports node fail/node_unverifiable naming the raw observed output, not node_missing or node_unsupported", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithStubNode("weird-build-2024")
			requireStubNodeVersion(path, "weird-build-2024")

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.SchemaVersion).NotTo(BeEmpty(), "the readiness document must still be produced on a probe failure")
			Expect(doc.Checks.Node.State).To(Equal("fail"))
			Expect(doc.Checks.Node.Code).To(Equal("node_unverifiable"))
			Expect(doc.Checks.Runtime.State).To(Equal("fail"))
			Expect(doc.Checks.Runtime.Code).To(Equal("node_unverifiable"))
			Expect(doc.Checks.Runtime.Detail).To(ContainSubstring("weird-build-2024"))
			Expect(doc.Checks.Runtime.Detail).NotTo(ContainSubstring(string(os.PathSeparator)), "the detail must not leak the stub node's host path")
			Expect(doc.Checks.Runtime.Version).To(BeEmpty(), "an unverifiable probe must not report a confirmed version")
			Expect(doc.Status).To(Equal("needs_prerequisite"))
		})
	})

	When("Node resolves and runs but prints an oversized unparsable version blob", func() {
		It("reports node fail/node_unverifiable with a detail bounded well below the raw probe-output budget", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`+"\n")

			path := pathWithOversizedUnparsableNode()

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Runtime.State).To(Equal("fail"))
			Expect(doc.Checks.Runtime.Code).To(Equal("node_unverifiable"))
			Expect(doc.Checks.Runtime.Detail).NotTo(BeEmpty())
			Expect(doc.Checks.Runtime.Detail).To(HavePrefix("node --version printed an unparsable version:"), "must land on the rawVersion-embedding branch, not the exceeded-probe-budget default")
			Expect(doc.Checks.Runtime.Detail).To(ContainSubstring("...(truncated)"), "the oversized rawVersion must actually have been truncated")
			Expect(len(doc.Checks.Runtime.Detail)).To(BeNumerically("<", 1<<10), "the probe-failure detail must be bounded even when the probed output is not")
			Expect(doc.Status).To(Equal("needs_prerequisite"))
		})
	})
})
