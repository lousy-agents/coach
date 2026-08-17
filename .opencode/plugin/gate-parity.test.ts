import assert from "node:assert"
import fs from "node:fs/promises"
import path from "node:path"
import test from "node:test"

import { PR_DIRTY_TREE_DENY_REASON } from "./implement-issue-gates.ts"

// The OpenCode gate and the Claude gate protect the same command file, which
// .opencode/plugin/claude-agents.ts mirrors verbatim. If they enforce different
// contracts, the prose promises something only one harness delivers.
//
// Both are now clean-worktree checks and nothing else. Since the `status`
// aggregator became a required check, a red tree cannot merge whatever either
// hook decides, so validating locally bought latency rather than safety. What
// survives is the one thing CI cannot do: CI validates the pushed commit, so
// only something local notices that the working tree differs from it and that
// the PR's evidence therefore describes neither.
test("neither gate runs a task runner any more", async () => {
  const repoRoot = path.resolve(path.join(import.meta.dirname, "..", ".."))
  const claudeGate = await fs.readFile(
    path.join(repoRoot, ".claude", "hooks", "gate-pr-creation.sh"), "utf8")
  assert.strictEqual(
    /^\s*if ! mise run /m.test(claudeGate), false,
    "the Claude gate invokes mise again; the OpenCode mirror would silently diverge")

  const source = await fs.readFile(
    path.join(import.meta.dirname, "implement-issue-gates.ts"), "utf8")
  assert.strictEqual(
    /spawnSync\("mise"/.test(source), false,
    "the OpenCode gate invokes mise again")
})

// The previous version of this test called worktreeIsClean directly and asserted
// it behaved. It does -- but the plugin never called it, so the OpenCode gate
// refused nothing at all while a test named for that refusal stayed green. A
// helper that works is not a gate that runs; drive the entry point.
test("the OpenCode PR gate refuses a tree that does not match what ships", async () => {
  const pluginModule = await import(new URL("./implement-issue-gates.ts", import.meta.url).href)
  const plugin = await pluginModule.default(
    { directory: "/tmp", worktree: "/tmp" },
    { checkWorktree: () => ({ ok: false, detail: " M some/file.go" }) },
  )

  await assert.rejects(
    () => plugin["tool.execute.before"](
      { tool: "bash", sessionID: "s", callID: "c" },
      { args: { command: "gh pr create --fill" } },
    ),
    (err: Error) => {
      assert.match(err.message, /working tree/i)
      return true
    },
    "a dirty tree must block PR creation through the plugin, not merely through a helper")
})

test("the OpenCode PR gate allows a clean tree", async () => {
  const pluginModule = await import(new URL("./implement-issue-gates.ts", import.meta.url).href)
  const plugin = await pluginModule.default(
    { directory: "/tmp", worktree: "/tmp" },
    { checkWorktree: () => ({ ok: true }) },
  )
  await plugin["tool.execute.before"](
    { tool: "bash", sessionID: "s", callID: "c" },
    { args: { command: "gh pr create --fill" } },
  )
})

test("both gates deny with the same reason", async () => {
  const repoRoot = path.resolve(path.join(import.meta.dirname, "..", ".."))
  const claudeGate = await fs.readFile(
    path.join(repoRoot, ".claude", "hooks", "gate-pr-creation.sh"), "utf8")
  assert.match(claudeGate, /working tree is dirty/i)
  assert.match(PR_DIRTY_TREE_DENY_REASON, /working tree is dirty/i)
})
