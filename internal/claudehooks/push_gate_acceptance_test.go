package claudehooks

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// runGateCommand drives gate-pr-creation.sh for an arbitrary tool name and
// shell command. runGate hardcodes `gh pr create`, which cannot express the
// push path.
func runGateCommand(toolName, command string, worktreeDirty, ciPasses bool) gateRun {
	GinkgoHelper()

	scriptPath, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "gate-pr-creation.sh"))
	Expect(err).NotTo(HaveOccurred())

	fakeBin := GinkgoT().TempDir()
	argsLog := filepath.Join(fakeBin, "mise-args.txt")

	exitCode := "0"
	if !ciPasses {
		exitCode = "1"
	}
	Expect(os.WriteFile(filepath.Join(fakeBin, "mise"),
		[]byte("#!/bin/sh\necho \"$@\" >> "+argsLog+"\nexit "+exitCode+"\n"), 0o755)).To(Succeed())

	porcelain := ""
	if worktreeDirty {
		porcelain = "echo ' M pkg/semantics/analyzer.go'\n"
	}
	Expect(os.WriteFile(filepath.Join(fakeBin, "git"),
		[]byte("#!/bin/sh\ncase \"$*\" in\n  'status --porcelain') "+porcelain+";;\nesac\nexit 0\n"), 0o755)).To(Succeed())

	payload, err := json.Marshal(map[string]any{
		"tool_name":  toolName,
		"tool_input": map[string]string{"command": command},
	})
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command("bash", scriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	Expect(cmd.Run()).To(Succeed(), "hook exited non-zero; stderr: %s", stderr.String())

	recorded, _ := os.ReadFile(argsLog)
	return gateRun{stdout: stdout.String(), miseArgs: string(recorded)}
}

// The PR gate fires on pull-request *creation*. Nothing fires on the push that
// precedes it, or on any of the repair pushes the implement-issue command sends
// after the PR is open -- so the commits that actually reach the remote were
// never checked against a clean tree or a green ci-gate. The PR body describes
// a tree the gate inspected; the branch can carry something else entirely.
//
// This matters more since the exhaustive suite moved to CI: the local gate's
// remaining job is tree identity, and tree identity is decided at push time.
var _ = Describe("the push gate", func() {
	When("a push would publish a tree that differs from what was validated", func() {
		It("denies the push", func() {
			expectDeny(runGateCommand("Bash", "git push -u origin my-branch", true, true),
				"working tree")
		})
	})

	When("a push follows a red smoke check", func() {
		It("denies the push", func() {
			expectDeny(runGateCommand("Bash", "git push -u origin my-branch", false, false),
				"ci-gate")
		})
	})

	When("the tree is clean and the smoke check is green", func() {
		It("allows the push, having actually run the check", func() {
			run := runGateCommand("Bash", "git push -u origin my-branch", false, true)
			Expect(run.stdout).To(BeEmpty(), "expected an allow, got: %s", run.stdout)
			Expect(run.miseArgs).To(ContainSubstring("run ci-gate"),
				"an allow that never ran the check is indistinguishable from an absent gate")
		})
	})

	// The Bash branch is a filter, not a catch-all. Widening it to every Bash
	// call would run ci-gate on `ls`, and the gate has to stay cheap enough that
	// nobody is tempted to remove it.
	When("a Bash command is not a publish operation", func() {
		It("passes through without running anything", func() {
			for _, command := range []string{"git status", "go test ./...", "git log --oneline"} {
				run := runGateCommand("Bash", command, true, true)
				Expect(run.stdout).To(BeEmpty(), "%q should not be gated", command)
				Expect(run.miseArgs).To(BeEmpty(), "%q should not have run mise", command)
			}
		})
	})

	// Observed, not hypothesised: a commit whose message discussed the push gate
	// was refused by the push gate. The hook sees one flat command string, so an
	// unanchored search cannot tell an invocation from a mention -- and in a
	// repository whose commit messages are largely about its own tooling, that
	// misfires constantly. Anchoring costs nothing: a false deny is safe but
	// infuriating, and the safe direction is already covered above.
	When("a command merely mentions a publish verb", func() {
		It("is not gated", func() {
			for _, command := range []string{
				`git commit -m "explain the git push gate"`,
				"echo 'run git push later' > notes.txt",
				`git commit -m "this also mentions gh pr create"`,
			} {
				run := runGateCommand("Bash", command, true, true)
				Expect(run.stdout).To(BeEmpty(), "%q is a mention, not an invocation", command)
			}
		})
	})

	When("a publish verb appears after a shell separator", func() {
		It("is still gated", func() {
			for _, command := range []string{
				"git add -A && git push",
				"make build; git push origin main",
				"git commit -m x && gh pr create --fill",
			} {
				expectDeny(runGateCommand("Bash", command, true, true), "working tree")
			}
		})
	})

	// git push is not the only way to write to the remote. On CCR the harness
	// supplies MCP tools that commit over the API with no shell involved, so a
	// Bash-only gate watches a door that is not the only door.
	When("the remote is written through the GitHub API instead of git", func() {
		It("denies those writes on a dirty tree too", func() {
			for _, tool := range []string{
				"mcp__github__push_files",
				"mcp__github__create_or_update_file",
				"mcp__github__delete_file",
			} {
				expectDeny(runGateCommand(tool, "", true, true), "working tree")
			}
		})
	})
})

// The gate is registered on an unguarded Bash matcher, so it now runs on every
// shell command in the session. Its liveness trace was written before the
// publish filter -- deliberately, so that "ran and did not match" stays
// distinguishable from "never registered" -- which under an unguarded matcher
// means every `ls` appends a row.
//
// That empties the trace of meaning, and the trace is what AGENTS.md tells a
// human to read to find out what a run actually did. The previous change to
// this repository was fixing pollution of that same file from a different
// direction, so restoring it from this one is not optional.
//
// The liveness signal survives: step 0's publish probe is itself a publish
// command, so it still traces.
var _ = Describe("the publish gate's trace", func() {
	tracePathFor := func() string {
		GinkgoHelper()
		return filepath.Join(GinkgoT().TempDir(), "trace.tsv")
	}

	When("an ordinary shell command runs", func() {
		It("writes nothing", func() {
			for _, command := range []string{"ls -la", "go test ./...", "git status"} {
				rows, _ := traceRows("gate-pr-creation.sh",
					map[string]any{"tool_name": "Bash", "tool_input": map[string]string{"command": command}},
					tracePathFor())
				Expect(rows).To(BeEmpty(),
					"%q traced a row; every command in the session would, and the trace stops being readable", command)
			}
		})
	})

	When("a command that publishes runs", func() {
		It("still traces, so the gate's liveness stays observable", func() {
			rows, _ := traceRows("gate-pr-creation.sh",
				map[string]any{"tool_name": "Bash", "tool_input": map[string]string{"command": "git push origin main"}},
				tracePathFor())
			Expect(rows).NotTo(BeEmpty(),
				"step 0 probes this path; with no row a dead gate and a quiet one look identical")
		})
	})

	When("the remote is written through the API", func() {
		It("traces, since those matchers are narrow enough not to flood", func() {
			rows, _ := traceRows("gate-pr-creation.sh",
				map[string]any{"tool_name": "mcp__github__push_files"},
				tracePathFor())
			Expect(rows).NotTo(BeEmpty())
		})
	})
})
