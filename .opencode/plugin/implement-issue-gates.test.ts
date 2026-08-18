import assert from "node:assert"
import test from "node:test"

import {
  extractTaskResultText,
  isValidReviewVerdict,
  needsReviewerFindingsRelay,
  RELAY_DENY_REASON,
  VERDICT_SOFT_FAIL_MESSAGE,
} from "./implement-issue-gates.ts"

test("isValidReviewVerdict: PASS — ok", () => {
  assert.strictEqual(isValidReviewVerdict("PASS — ok"), true)
})

test("isValidReviewVerdict: FINDINGS with heading", () => {
  assert.strictEqual(
    isValidReviewVerdict("FINDINGS\n\n## Reviewer Findings\n1. x"),
    true,
  )
})

test("isValidReviewVerdict: mid-message PASS is invalid", () => {
  assert.strictEqual(isValidReviewVerdict("I reviewed.\nPASS — done"), false)
})

test("isValidReviewVerdict: prose only is invalid", () => {
  assert.strictEqual(isValidReviewVerdict("Looks fine to me."), false)
})

test("isValidReviewVerdict: leading blank lines then PASS is valid", () => {
  assert.strictEqual(isValidReviewVerdict("\n\nPASS — verified\n"), true)
})

test("isValidReviewVerdict: empty is invalid", () => {
  assert.strictEqual(isValidReviewVerdict(""), false)
})

test("isValidReviewVerdict: PASSWORD does not count (word boundary)", () => {
  assert.strictEqual(isValidReviewVerdict("PASSWORD reset needed"), false)
})

test("isValidReviewVerdict: mid-message FINDINGS is invalid", () => {
  assert.strictEqual(isValidReviewVerdict("Here is my review.\nFINDINGS\n1. foo"), false)
})

test("extractTaskResultText: strips task wrapper", () => {
  const wrapped = `<task id="abc" state="completed">
<task_result>
PASS — ok
</task_result>
</task>`
  assert.strictEqual(extractTaskResultText(wrapped).trim(), "PASS — ok")
})

test("extractTaskResultText: plain text passthrough", () => {
  assert.strictEqual(extractTaskResultText("FINDINGS\n\nx"), "FINDINGS\n\nx")
})

test("needsReviewerFindingsRelay: rework without heading", () => {
  assert.strictEqual(
    needsReviewerFindingsRelay(
      "task-implementer",
      "Re-delegating after the reviewer's findings, please fix the issues.",
    ),
    true,
  )
})

test("needsReviewerFindingsRelay: rework with heading is ok", () => {
  assert.strictEqual(
    needsReviewerFindingsRelay(
      "task-implementer",
      "Re-delegating after FINDINGS.\n\n## Reviewer Findings\n1. foo.go:12 fix bar",
    ),
    false,
  )
})

test("needsReviewerFindingsRelay: domain findings word alone is ok", () => {
  assert.strictEqual(
    needsReviewerFindingsRelay(
      "task-implementer",
      "Implement task 3: extend Result.Findings for the new metric.",
    ),
    false,
  )
})

test("needsReviewerFindingsRelay: non-implementer is ok", () => {
  assert.strictEqual(
    needsReviewerFindingsRelay(
      "task-reviewer",
      "Re-delegating after reviewer findings, no heading included.",
    ),
    false,
  )
})

test("needsReviewerFindingsRelay: re-delegate wording without heading", () => {
  assert.strictEqual(
    needsReviewerFindingsRelay("task-implementer", "re-review needed, redelegating this task."),
    true,
  )
})

test("hooks: implementer rework without findings throws", async () => {
  const pluginModule = await import(new URL("./implement-issue-gates.ts", import.meta.url).href)
  const plugin = await pluginModule.default({ directory: "/tmp", worktree: "/tmp" })

  await assert.rejects(
    () =>
      plugin["tool.execute.before"](
        { tool: "task", sessionID: "s", callID: "c" },
        {
          args: {
            subagent_type: "task-implementer",
            prompt: "Please re-delegate and fix reviewer findings",
          },
        },
      ),
    (err: Error) => {
      assert.ok(err instanceof Error)
      assert.strictEqual(err.message, RELAY_DENY_REASON)
      return true
    },
  )
})

// The plugin used to gate PR creation on a clean worktree. That gate is gone
// on purpose — publish safety is branch protection plus the required `status`
// check — so a bash tool call must pass through untouched.
test("hooks: bash tool calls are not gated", async () => {
  const pluginModule = await import(new URL("./implement-issue-gates.ts", import.meta.url).href)
  const plugin = await pluginModule.default({ directory: "/tmp", worktree: "/tmp" })

  await plugin["tool.execute.before"](
    { tool: "bash", sessionID: "s", callID: "c" },
    { args: { command: "gh pr create --title x --body y" } },
  )
})

test("hooks: malformed reviewer verdict is soft-rewritten", async () => {
  const pluginModule = await import(new URL("./implement-issue-gates.ts", import.meta.url).href)
  const plugin = await pluginModule.default({ directory: "/tmp", worktree: "/tmp" })

  const output = {
    title: "task",
    output: `<task id="x" state="completed"><task_result>
I reviewed the diff.
PASS — done
</task_result></task>`,
    metadata: {},
  }
  await plugin["tool.execute.after"](
    {
      tool: "task",
      sessionID: "s",
      callID: "c",
      args: { subagent_type: "task-reviewer" },
    },
    output,
  )
  assert.strictEqual(output.output, VERDICT_SOFT_FAIL_MESSAGE)
})

test("hooks: valid reviewer PASS is left alone", async () => {
  const pluginModule = await import(new URL("./implement-issue-gates.ts", import.meta.url).href)
  const plugin = await pluginModule.default({ directory: "/tmp", worktree: "/tmp" })

  const original = "PASS — verified the diff and tests."
  const output = { title: "task", output: original, metadata: {} }
  await plugin["tool.execute.after"](
    {
      tool: "task",
      sessionID: "s",
      callID: "c",
      args: { subagent_type: "task-reviewer" },
    },
    output,
  )
  assert.strictEqual(output.output, original)
})
