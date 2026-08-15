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
		It("validates with the authoritative ci-all task, not the narrower ci", func() {
			run := runGate("mcp__github__create_pull_request", false, true)
			Expect(run.miseArgs).To(ContainSubstring("run ci-all"),
				"ci alone leaves wasm-build uncovered and lets pkg/projectmodel's sidecar suite skip silently")
		})

		It("allows the pull request when that task passes", func() {
			Expect(strings.TrimSpace(runGate("mcp__github__create_pull_request", false, true).stdout)).To(BeEmpty())
		})

		It("denies the pull request when that task fails", func() {
			expectDeny(runGate("mcp__github__create_pull_request", false, false), "ci-all")
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

	When("the tool call is not a pull-request creation", func() {
		It("stays out of the way entirely", func() {
			run := runGate("Read", true, false)
			Expect(strings.TrimSpace(run.stdout)).To(BeEmpty())
			Expect(run.miseArgs).To(BeEmpty())
		})
	})
})
