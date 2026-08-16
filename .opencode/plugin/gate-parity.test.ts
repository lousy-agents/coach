import assert from "node:assert"
import fs from "node:fs/promises"
import path from "node:path"
import test from "node:test"

import { PR_CI_DENY_REASON, runMiseCiAll } from "./implement-issue-gates.ts"

// The OpenCode gate and the Claude gate protect the same command file, which
// .opencode/plugin/claude-agents.ts mirrors verbatim. Its step 4 states that
// ci-all is the first place wasm-build, the sidecar-built projectmodel suite,
// and cross-file gofmt/tidy-check touch the integrated tree -- so a gate here
// running plain `ci` lets an OpenCode run open a PR on a broken wasm build or
// an unrun sidecar suite, while the prose promises otherwise.
test("the OpenCode PR gate runs the same task the Claude gate runs", async () => {
  const repoRoot = path.resolve(path.join(import.meta.dirname, "..", ".."))
  const claudeGate = await fs.readFile(
    path.join(repoRoot, ".claude", "hooks", "gate-pr-creation.sh"), "utf8")
  const claudeTask = claudeGate.match(/^if ! mise run (ci[a-z-]*);/m)?.[1]
  assert.ok(claudeTask, "could not read the task the Claude gate runs")

  const source = await fs.readFile(
    path.join(import.meta.dirname, "implement-issue-gates.ts"), "utf8")
  const openCodeTask = source.match(/spawnSync\("mise", \["run", "([^"]+)"\]/)?.[1]
  assert.strictEqual(openCodeTask, claudeTask,
    `OpenCode gate runs "${openCodeTask}" but the Claude gate runs "${claudeTask}"`)

  assert.match(PR_CI_DENY_REASON, new RegExp(claudeTask),
    "the deny reason must name the task actually run, or it misdirects the fix")
  assert.ok(typeof runMiseCiAll === "function")
})
