/**
 * Mechanical gates for the implement-issue orchestrator flow in OpenCode.
 * Mirrors Claude hooks under .claude/hooks/ (shape only; no semantic re-judgment).
 */
import { spawnSync } from "node:child_process"

type PluginInput = {
  directory: string
  worktree: string
  $?: (...args: unknown[]) => { cwd: (dir: string) => Promise<unknown> }
  client?: {
    app?: {
      log?: (input: {
        body: {
          service: string
          level: "debug" | "info" | "warn" | "error"
          message: string
          extra?: Record<string, unknown>
        }
      }) => Promise<void>
    }
  }
}

/** First non-empty line must be PASS or FINDINGS (word-boundary). Mid-message verdicts fail. */
export function isValidReviewVerdict(text: string): boolean {
  const firstLine = firstNonEmptyLine(text)
  return /^PASS\b/.test(firstLine) || /^FINDINGS\b/.test(firstLine)
}

export function firstNonEmptyLine(text: string): string {
  for (const line of text.split(/\r?\n/)) {
    if (/[^\s]/.test(line)) return line
  }
  return ""
}

/** Strip OpenCode task tool wrapper when present so the verdict check sees assistant text. */
export function extractTaskResultText(output: string): string {
  const match = output.match(/<task_result>\s*([\s\S]*?)\s*<\/task_result>/i)
  if (match) return match[1]
  return output
}

/** True when the tool call is a PR-create attempt (bash gh or clear MCP create_pull_request). */
export function isPrCreateAttempt(tool: string, args: { command?: unknown } | null | undefined): boolean {
  const name = tool ?? ""
  if (name === "bash" || name === "Bash") {
    const command = typeof args?.command === "string" ? args.command : ""
    if (!command) return false
    return /\bgh\s+pr\s+create\b/.test(command)
  }
  if (name === "mcp__github__create_pull_request") return true
  if (/(^|__)create_pull_request$/.test(name)) return true
  return false
}

/**
 * True when a task-implementer rework prompt must include "## Reviewer Findings".
 * Presence-only; does not validate findings content.
 */
export function needsReviewerFindingsRelay(
  subagentType: string | undefined,
  prompt: string | undefined,
): boolean {
  if (subagentType !== "task-implementer") return false
  const p = prompt ?? ""
  if (!/reviewer.{0,40}finding|re-?delegat|re-review/i.test(p)) return false
  return !p.includes("## Reviewer Findings")
}

export const RELAY_DENY_REASON =
  'Re-delegation after FINDINGS must include the reviewer\'s "## Reviewer Findings" block verbatim, not a paraphrase.'

export const PR_DIRTY_TREE_DENY_REASON =
  "The working tree is dirty, so validation would not describe the commit this PR publishes. Commit or stash the changes, then retry."
// Kept as an alias so a mirrored prompt referring to it still resolves; the
// gate has only one denial now.
export const PR_CI_DENY_REASON = PR_DIRTY_TREE_DENY_REASON

export const VERDICT_SOFT_FAIL_MESSAGE = [
  "ERROR: task-reviewer reply shape invalid.",
  "The first non-empty line of the reviewer result must be PASS or FINDINGS (word boundary).",
  "Do not invent a PASS. Re-delegate task-reviewer (fresh or continued) until the reply begins with PASS or FINDINGS verbatim.",
].join(" ")

export type WorktreeCheck = (cwd: string) => { ok: boolean; detail?: string }

// Mirrors gate-pr-creation.sh's first check. The suite validates the working
// tree while a pull request publishes committed history; if they differ, the PR
// ships a tree nothing validated. A git failure denies rather than passing --
// "git is broken" and "the tree is clean" must not be the same observation.
export function worktreeIsClean(cwd: string): { ok: boolean; detail?: string } {
  const result = spawnSync("git", ["status", "--porcelain"], { cwd, encoding: "utf8", env: process.env })
  if (result.error) return { ok: false, detail: result.error.message }
  if (result.status !== 0) return { ok: false, detail: (result.stderr || "").trim() || `exit ${result.status}` }
  const dirty = (result.stdout || "").trim()
  return dirty ? { ok: false, detail: dirty.split("\n").slice(0, 10).join("\n") } : { ok: true }
}

async function log(
  client: PluginInput["client"],
  level: "debug" | "info" | "warn" | "error",
  message: string,
  extra?: Record<string, unknown>,
) {
  try {
    await client?.app?.log?.({
      body: { service: "implement-issue-gates", level, message, extra },
    })
  } catch {
    // ignore logging failures
  }
}

export type GatesOptions = {
  checkWorktree?: WorktreeCheck
}

export default async (input: PluginInput, options: GatesOptions = {}) => {
  const checkWorktree = options.checkWorktree ?? worktreeIsClean
  const cwd = input.worktree || input.directory

  return {
    "tool.execute.before": async (
      hookInput: { tool: string; sessionID: string; callID: string },
      output: { args: Record<string, unknown> },
    ) => {
      const tool = hookInput.tool
      const args = output.args ?? {}

      if (tool === "task") {
        const subagentType =
          typeof args.subagent_type === "string" ? args.subagent_type : undefined
        const prompt = typeof args.prompt === "string" ? args.prompt : undefined
        if (needsReviewerFindingsRelay(subagentType, prompt)) {
          await log(input.client, "warn", "blocked implementer rework without findings relay", {
            sessionID: hookInput.sessionID,
          })
          throw new Error(RELAY_DENY_REASON)
        }
      }

      // The Claude hook's only remaining check, mirrored: a pull request
      // publishes committed history, so a dirty tree means its evidence
      // describes something that was never pushed. Everything else is a
      // required CI check now.
      if (isPrCreateAttempt(tool, args as { command?: unknown })) {
        const result = checkWorktree(cwd)
        if (!result.ok) {
          await log(input.client, "warn", "blocked PR create; dirty worktree", {
            sessionID: hookInput.sessionID,
            detail: result.detail,
          })
          throw new Error(PR_DIRTY_TREE_DENY_REASON)
        }
      }
    },

    "tool.execute.after": async (
      hookInput: { tool: string; sessionID: string; callID: string; args?: Record<string, unknown> },
      output: { title: string; output: string; metadata: unknown },
    ) => {
      if (hookInput.tool !== "task") return
      const args = hookInput.args ?? {}
      if (args.subagent_type !== "task-reviewer") return

      const raw = typeof output.output === "string" ? output.output : ""
      const text = extractTaskResultText(raw)
      if (isValidReviewVerdict(text)) return

      await log(input.client, "warn", "soft-failed malformed task-reviewer verdict", {
        sessionID: hookInput.sessionID,
        firstLine: firstNonEmptyLine(text),
      })
      output.output = VERDICT_SOFT_FAIL_MESSAGE
    },
  }
}
