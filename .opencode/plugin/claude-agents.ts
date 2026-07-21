/**
 * Loads Claude Code subagents from .claude/agents/*.md into OpenCode.
 * Canonical source: .claude/agents/ — do not mirror prompt bodies here.
 */
import fs from "node:fs/promises"
import path from "node:path"

type PermissionAction = "allow" | "ask" | "deny"
type AgentConfig = {
  description?: string
  mode?: "subagent" | "primary" | "all"
  prompt?: string
  steps?: number
  permission?: Record<string, PermissionAction | Record<string, PermissionAction>>
  [key: string]: unknown
}

type Config = {
  agent?: Record<string, AgentConfig>
  [key: string]: unknown
}

type PluginInput = {
  directory: string
  worktree: string
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

const CLAUDE_TOOL_TO_PERM: Record<string, string> = {
  Read: "read",
  Grep: "grep",
  Glob: "glob",
  Bash: "bash",
  Edit: "edit",
  Write: "edit",
}

const PERM_KEYS = ["read", "edit", "glob", "grep", "list", "bash"] as const

/** Matches .claude/scripts/validate-no-git-writes.sh intent via OpenCode bash permissions. */
const GIT_WRITE_DENY: Record<string, PermissionAction> = {
  "*": "allow",
  "git commit*": "deny",
  "git push*": "deny",
  "git merge*": "deny",
  "git rebase*": "deny",
  "git reset*": "deny",
  "git cherry-pick*": "deny",
  "git tag*": "deny",
  "gh pr create*": "deny",
  "gh pr merge*": "deny",
  "gh pr edit*": "deny",
  "gh pr close*": "deny",
  "gh release create*": "deny",
  "gh release edit*": "deny",
  "gh release delete*": "deny",
}

function parseFrontmatter(text: string): { data: Record<string, string>; body: string } | null {
  const normalized = text.replace(/^\uFEFF/, "")
  if (!normalized.startsWith("---\n") && !normalized.startsWith("---\r\n")) return null

  const afterOpen = normalized.startsWith("---\r\n") ? 5 : 4
  const rest = normalized.slice(afterOpen)
  const endMatch = rest.match(/\r?\n---\r?\n/)
  if (!endMatch || endMatch.index === undefined) return null

  const fm = rest.slice(0, endMatch.index)
  const body = rest.slice(endMatch.index + endMatch[0].length)
  const data: Record<string, string> = {}
  let key: string | null = null
  let buf: string[] = []

  const flush = () => {
    if (key !== null) {
      data[key] = buf.join("\n").trim()
      key = null
      buf = []
    }
  }

  for (const line of fm.split(/\r?\n/)) {
    if (key !== null && (/^\s/.test(line) || line.startsWith("- "))) {
      buf.push(line)
      continue
    }
    flush()
    const m = line.match(/^([A-Za-z0-9_-]+):\s*(.*)$/)
    if (!m) continue
    const [, k, raw] = m
    if (raw === "" || raw === "|" || raw === ">") {
      key = k
      buf = []
    } else {
      data[k] = raw.replace(/^["']|["']$/g, "")
    }
  }
  flush()
  return { data, body }
}

function parseTools(raw: string | undefined): string[] {
  if (!raw) return []
  return raw
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean)
}

function permissionsFromTools(
  tools: string[],
  blockGitWrites: boolean,
): Record<string, PermissionAction | Record<string, PermissionAction>> {
  const allowed = new Set<string>()
  for (const tool of tools) {
    const perm = CLAUDE_TOOL_TO_PERM[tool]
    if (perm) allowed.add(perm)
  }
  if (allowed.has("glob") || allowed.has("read") || allowed.has("grep")) {
    allowed.add("list")
  }

  const permission: Record<string, PermissionAction | Record<string, PermissionAction>> = {}
  for (const key of PERM_KEYS) {
    if (key === "bash" && blockGitWrites && allowed.has("bash")) {
      permission.bash = { ...GIT_WRITE_DENY }
      continue
    }
    permission[key] = allowed.has(key) ? "allow" : "deny"
  }
  permission.todowrite = "deny"
  return permission
}

function hasGitWriteHook(data: Record<string, string>): boolean {
  const hooks = data.hooks ?? ""
  return (
    hooks.includes("validate-no-git-writes") ||
    /PreToolUse/i.test(hooks) && /matcher:.*Bash/i.test(hooks)
  )
}

function agentName(data: Record<string, string>, filePath: string): string {
  const fromFm = data.name?.trim()
  if (fromFm) return fromFm
  return path.basename(filePath, path.extname(filePath))
}

async function loadClaudeAgents(agentsDir: string): Promise<Record<string, AgentConfig>> {
  const out: Record<string, AgentConfig> = {}
  let entries: string[]
  try {
    entries = (await fs.readdir(agentsDir)).sort()
  } catch {
    return out
  }

  for (const entry of entries) {
    if (!entry.endsWith(".md")) continue
    const filePath = path.join(agentsDir, entry)
    let text: string
    try {
      text = await fs.readFile(filePath, "utf8")
    } catch {
      continue
    }

    const parsed = parseFrontmatter(text)
    if (!parsed || !parsed.body) continue

    const name = agentName(parsed.data, filePath)
    const tools = parseTools(parsed.data.tools)
    const blockGitWrites = hasGitWriteHook(parsed.data)
    const maxTurns = parsed.data.maxTurns ? Number.parseInt(parsed.data.maxTurns, 10) : undefined

    const agent: AgentConfig = {
      description: parsed.data.description?.trim() || `Claude agent ${name}`,
      mode: "subagent",
      prompt: parsed.body,
      permission: permissionsFromTools(tools, blockGitWrites),
    }
    if (Number.isFinite(maxTurns) && maxTurns! > 0) {
      agent.steps = maxTurns
    }
    out[name] = agent
  }
  return out
}

async function log(
  client: PluginInput["client"],
  level: "debug" | "info" | "warn" | "error",
  message: string,
  extra?: Record<string, unknown>,
) {
  try {
    await client?.app?.log?.({
      body: { service: "claude-agents", level, message, extra },
    })
  } catch {
    // ignore logging failures
  }
}

export default async (input: PluginInput) => {
  return {
    config: async (cfg: Config) => {
      const roots = [input.worktree, input.directory].filter(Boolean)
      const seen = new Set<string>()
      const loaded: Record<string, AgentConfig> = {}

      for (const root of roots) {
        const agentsDir = path.join(root, ".claude", "agents")
        if (seen.has(agentsDir)) continue
        seen.add(agentsDir)
        Object.assign(loaded, await loadClaudeAgents(agentsDir))
      }

      const names = Object.keys(loaded)
      if (names.length === 0) {
        await log(input.client, "debug", "no Claude agents found under .claude/agents")
        return
      }

      cfg.agent = cfg.agent ?? {}
      let injected = 0
      for (const [name, agent] of Object.entries(loaded)) {
        // Explicit OpenCode agent defs win over the Claude loader.
        if (cfg.agent[name] !== undefined) continue
        cfg.agent[name] = agent
        injected++
      }

      await log(input.client, "info", "loaded Claude agents into OpenCode", {
        found: names.length,
        injected,
        names,
      })
    },
  }
}
