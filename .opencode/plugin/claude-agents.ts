/**
 * Loads Claude Code subagents and slash commands into OpenCode.
 * Canonical sources: .claude/agents/ and .claude/commands/ — do not mirror bodies.
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

type CommandConfig = {
  description?: string
  template?: string
  agent?: string
  model?: string
  subtask?: boolean
  [key: string]: unknown
}

type Config = {
  agent?: Record<string, AgentConfig>
  command?: Record<string, CommandConfig>
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
    const maxTurnsRaw =
      parsed.data.maxTurns ?? parsed.data.max_turns ?? parsed.data["max-turns"]
    const maxTurns = maxTurnsRaw ? Number.parseInt(maxTurnsRaw, 10) : undefined

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

function commandName(data: Record<string, string>, filePath: string): string {
  const fromFm = data.name?.trim()
  if (fromFm) return fromFm
  return path.basename(filePath, path.extname(filePath))
}

async function loadClaudeCommands(commandsDir: string): Promise<Record<string, CommandConfig>> {
  const out: Record<string, CommandConfig> = {}
  let entries: string[]
  try {
    entries = (await fs.readdir(commandsDir)).sort()
  } catch {
    return out
  }

  for (const entry of entries) {
    if (!entry.endsWith(".md")) continue
    const filePath = path.join(commandsDir, entry)
    let text: string
    try {
      text = await fs.readFile(filePath, "utf8")
    } catch {
      continue
    }

    const parsed = parseFrontmatter(text)
    if (!parsed) continue

    const name = commandName(parsed.data, filePath)
    // Ignore Claude-only frontmatter: argument-hint, model: inherit (omit model).
    out[name] = {
      description: parsed.data.description?.trim() || `Claude command ${name}`,
      template: parsed.body,
    }
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
      const seenAgents = new Set<string>()
      const seenCommands = new Set<string>()
      const loadedAgents: Record<string, AgentConfig> = {}
      const loadedCommands: Record<string, CommandConfig> = {}

      for (const root of roots) {
        const agentsDir = path.join(root, ".claude", "agents")
        if (!seenAgents.has(agentsDir)) {
          seenAgents.add(agentsDir)
          Object.assign(loadedAgents, await loadClaudeAgents(agentsDir))
        }
        const commandsDir = path.join(root, ".claude", "commands")
        if (!seenCommands.has(commandsDir)) {
          seenCommands.add(commandsDir)
          Object.assign(loadedCommands, await loadClaudeCommands(commandsDir))
        }
      }

      const agentNames = Object.keys(loadedAgents)
      if (agentNames.length === 0) {
        await log(input.client, "debug", "no Claude agents found under .claude/agents")
      } else {
        cfg.agent = cfg.agent ?? {}
        let injected = 0
        for (const [name, agent] of Object.entries(loadedAgents)) {
          // Explicit OpenCode agent defs win over the Claude loader.
          if (cfg.agent[name] !== undefined) continue
          cfg.agent[name] = agent
          injected++
        }

        await log(input.client, "info", "loaded Claude agents into OpenCode", {
          found: agentNames.length,
          injected,
          names: agentNames,
        })
      }

      const commandNames = Object.keys(loadedCommands)
      if (commandNames.length === 0) {
        await log(input.client, "debug", "no Claude commands found under .claude/commands")
      } else {
        cfg.command = cfg.command ?? {}
        let injected = 0
        for (const [name, command] of Object.entries(loadedCommands)) {
          // Explicit OpenCode command defs win over the Claude loader.
          if (cfg.command[name] !== undefined) continue
          cfg.command[name] = command
          injected++
        }

        await log(input.client, "info", "loaded Claude commands into OpenCode", {
          found: commandNames.length,
          injected,
          names: commandNames,
        })
      }
    },
  }
}
