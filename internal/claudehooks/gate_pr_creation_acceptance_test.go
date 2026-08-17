package claudehooks

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// gateRun is one synthetic PreToolUse invocation of gate-pr-creation.sh with
// `mise` and `git` stubbed on PATH, so the specs never run the real suite and
// never touch a real repository. miseArgs records what the hook asked mise to
// run, which is how the specs assert the gate validates the authoritative
// task rather than the narrower one.
type gateRun struct {
	stdout   string
	miseArgs string
}

func runGate(toolName string, worktreeDirty, ciPasses bool) gateRun {
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

	// `git status --porcelain` is the only git the gate needs: non-empty
	// output means the working tree differs from HEAD, so whatever the suite
	// validated is not what a commit would publish.
	porcelain := ""
	if worktreeDirty {
		porcelain = "echo ' M pkg/semantics/analyzer.go'\n"
	}
	Expect(os.WriteFile(filepath.Join(fakeBin, "git"),
		[]byte("#!/bin/sh\ncase \"$*\" in\n  'status --porcelain') "+porcelain+";;\nesac\nexit 0\n"), 0o755)).To(Succeed())

	payload, err := json.Marshal(map[string]any{
		"tool_name":  toolName,
		"tool_input": map[string]string{"command": "gh pr create --title x --body y"},
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

// runGateWithBrokenGit stubs git as failing outright, standing in for a
// corrupt repository, a missing binary, or a permissions failure.
func runGateWithBrokenGit() gateRun {
	GinkgoHelper()

	scriptPath, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "gate-pr-creation.sh"))
	Expect(err).NotTo(HaveOccurred())

	fakeBin := GinkgoT().TempDir()
	argsLog := filepath.Join(fakeBin, "mise-args.txt")
	Expect(os.WriteFile(filepath.Join(fakeBin, "mise"),
		[]byte("#!/bin/sh\necho \"$@\" >> "+argsLog+"\nexit 0\n"), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(fakeBin, "git"),
		[]byte("#!/bin/sh\necho 'fatal: not a git repository' >&2\nexit 128\n"), 0o755)).To(Succeed())

	payload, err := json.Marshal(map[string]any{
		"tool_name":  "mcp__github__create_pull_request",
		"tool_input": map[string]string{"command": ""},
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

func expectDeny(run gateRun, reasonSubstring string) {
	GinkgoHelper()
	var payload struct {
		HookSpecificOutput struct {
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	Expect(json.Unmarshal([]byte(run.stdout), &payload)).To(Succeed(),
		"expected deny JSON on stdout, got %q", run.stdout)
	Expect(payload.HookSpecificOutput.PermissionDecision).To(Equal("deny"))
	Expect(payload.HookSpecificOutput.PermissionDecisionReason).To(ContainSubstring(reasonSubstring))
}

var _ = Describe("gate-pr-creation", func() {
	When("a pull request is being opened and the working tree is clean", func() {
		// Was ci-all. GitHub Actions now runs the same checks as parallel
		// required jobs -- a strict superset, including platform-smoke -- in
		// less wall clock than the serial local run, and on compute that is not
		// the session's. Re-running it here proved less, cost more, and pushed
		// against the hook's own 900s timeout. What is left is the part GHA
		// structurally cannot do (the clean-tree check above) plus a seconds-
		// scale smoke signal.
		It("validates with the cheap ci-gate task, leaving the suite to GHA", func() {
			run := runGate("mcp__github__create_pull_request", false, true)
			Expect(run.miseArgs).To(ContainSubstring("run ci-gate"),
				"a gate that re-runs the full suite duplicates GHA at twice the wall clock on scarcer compute")
			Expect(run.miseArgs).NotTo(ContainSubstring("run ci-all"),
				"ci-all here is the design this replaced; it is only safe to drop because GHA requires those jobs")
		})

		It("allows the pull request when that task passes", func() {
			Expect(strings.TrimSpace(runGate("mcp__github__create_pull_request", false, true).stdout)).To(BeEmpty())
		})

		It("denies the pull request when that task fails", func() {
			expectDeny(runGate("mcp__github__create_pull_request", false, false), "ci-gate")
		})
	})

	// The suite validates the working tree, but a pull request publishes
	// committed history. If those differ, the PR ships a tree nothing
	// validated -- a partial `git add`, or a stray untracked file.
	When("the working tree does not match what a commit would publish", func() {
		It("denies the pull request", func() {
			expectDeny(runGate("mcp__github__create_pull_request", true, true), "working tree")
		})

		It("does so without paying for the full suite first", func() {
			Expect(runGate("mcp__github__create_pull_request", true, true).miseArgs).To(BeEmpty(),
				"the cheap check must short-circuit before the expensive one")
		})
	})

	// Swallowing a git failure would make "git is broken or missing" and "the
	// tree is clean" the same observation, and the gate would allow a PR having
	// verified nothing -- the fail-open shape AGENTS.md's store/dependency
	// policy exists to prevent.
	When("the working tree's state cannot be determined at all", func() {
		It("refuses rather than assuming the tree is clean", func() {
			expectDeny(runGateWithBrokenGit(), "Could not determine")
		})

		It("does not fall through to the suite", func() {
			Expect(runGateWithBrokenGit().miseArgs).To(BeEmpty())
		})
	})

	When("the tool call is not a pull-request creation", func() {
		It("stays out of the way entirely", func() {
			run := runGate("Read", true, false)
			Expect(strings.TrimSpace(run.stdout)).To(BeEmpty())
			Expect(run.miseArgs).To(BeEmpty())
		})
	})
})
