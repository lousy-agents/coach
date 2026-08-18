/**
 * Review-loop fidelity checks for the implement-issue orchestrator flow in
 * OpenCode. Mirrors the two Claude hooks under .claude/hooks/ (shape only; no
 * semantic re-judgment): verify-context-relay.sh and verify-review-verdict.sh.
 * Publish safety is branch protection plus the required `status` check — this
 * plugin deliberately gates nothing about git or pull requests.
 */

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

export const VERDICT_SOFT_FAIL_MESSAGE = [
  "ERROR: task-reviewer reply shape invalid.",
  "The first non-empty line of the reviewer result must be PASS or FINDINGS (word boundary).",
  "Do not invent a PASS. Re-delegate task-reviewer (fresh or continued) until the reply begins with PASS or FINDINGS verbatim.",
].join(" ")

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

export default async (input: PluginInput) => {
  return {
    "tool.execute.before": async (
      hookInput: { tool: string; sessionID: string; callID: string },
      output: { args: Record<string, unknown> },
    ) => {
      if (hookInput.tool !== "task") return
      const args = output.args ?? {}
      const subagentType =
        typeof args.subagent_type === "string" ? args.subagent_type : undefined
      const prompt = typeof args.prompt === "string" ? args.prompt : undefined
      if (needsReviewerFindingsRelay(subagentType, prompt)) {
        await log(input.client, "warn", "blocked implementer rework without findings relay", {
          sessionID: hookInput.sessionID,
        })
        throw new Error(RELAY_DENY_REASON)
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
