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

function parseFrontmatter(text: string): { data: Record<string, string>; body: string } | null {
  const normalized = text.replace(/^\uFEFF/, "")
  if (!normalized.startsWith("---\n") && !normalized.startsWith("---\r\n")) return null

  const afterOpen = normalized.startsWith("---\r\n") ? 5 : 4
  const rest = normalized.slice(afterOpen)
  const endMatch = rest.match(/\r?\n---\r?\n/)
  if (!endMatch || endMatch.index === undefined) return null

  const fm = rest.slice(0, endMatch.index)
  const body = rest.slice(endMatch.index + endMatch[0].length)
  return { data: parseFrontmatterBlock(fm), body }
}

class FrontmatterBlock {
  readonly data: Record<string, string> = {}
  private key: string | null = null
  private buf: string[] = []

  addLine(line: string): void {
    if (this.key !== null && (/^\s/.test(line) || line.startsWith("- "))) {
      this.buf.push(line)
      return
    }
    this.flush()
    const m = line.match(/^([A-Za-z0-9_-]+):\s*(.*)$/)
    if (!m) return
    const [, k, raw] = m
    if (raw === "" || raw === "|" || raw === ">") {
      this.key = k
      this.buf = []
      return
    }
    this.data[k] = raw.replace(/^["']|["']$/g, "")
  }

  finish(): Record<string, string> {
    this.flush()
    return this.data
  }

  private flush(): void {
    if (this.key === null) return
    this.data[this.key] = this.buf.join("\n").trim()
    this.key = null
    this.buf = []
  }
}

function parseFrontmatterBlock(fm: string): Record<string, string> {
  const block = new FrontmatterBlock()
  for (const line of fm.split(/\r?\n/)) block.addLine(line)
  return block.finish()
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
    permission[key] = allowed.has(key) ? "allow" : "deny"
  }
  permission.todowrite = "deny"
  return permission
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
    const loaded = await loadClaudeAgentFile(path.join(agentsDir, entry))
    if (loaded) out[loaded.name] = loaded.agent
  }
  return out
}

async function loadClaudeAgentFile(filePath: string): Promise<{ name: string; agent: AgentConfig } | null> {
  let text: string
  try {
    text = await fs.readFile(filePath, "utf8")
  } catch {
    return null
  }
  const parsed = parseFrontmatter(text)
  if (!parsed || !parsed.body) return null
  const name = agentName(parsed.data, filePath)
  const tools = parseTools(parsed.data.tools)
  const maxTurnsRaw = parsed.data.maxTurns ?? parsed.data.max_turns ?? parsed.data["max-turns"]
  const maxTurns = maxTurnsRaw ? Number.parseInt(maxTurnsRaw, 10) : undefined
  const agent: AgentConfig = {
    description: parsed.data.description?.trim() || `Claude agent ${name}`,
    mode: "subagent",
    prompt: parsed.body,
    permission: permissionsFromTools(tools),
  }
  if (Number.isFinite(maxTurns) && maxTurns! > 0) {
    agent.steps = maxTurns
  }
  return { name, agent }
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

class ConfigDraft {
  cfg: Config
  constructor(cfg: Config) {
    this.cfg = cfg
  }

  mergeAgents(loaded: Record<string, AgentConfig>): { found: number; injected: number; names: string[] } {
    const names = Object.keys(loaded)
    if (names.length === 0) return { found: 0, injected: 0, names }
    const agent = { ...(this.cfg.agent ?? {}) }
    let injected = 0
    for (const [name, value] of Object.entries(loaded)) {
      // Explicit OpenCode agent defs win over the Claude loader.
      if (agent[name] !== undefined) continue
      agent[name] = value
      injected++
    }
    this.cfg.agent = agent
    return { found: names.length, injected, names }
  }

  mergeCommands(loaded: Record<string, CommandConfig>): { found: number; injected: number; names: string[] } {
    const names = Object.keys(loaded)
    if (names.length === 0) return { found: 0, injected: 0, names }
    const command = { ...(this.cfg.command ?? {}) }
    let injected = 0
    for (const [name, value] of Object.entries(loaded)) {
      // Explicit OpenCode command defs win over the Claude loader.
      if (command[name] !== undefined) continue
      command[name] = value
      injected++
    }
    this.cfg.command = command
    return { found: names.length, injected, names }
  }
}

async function injectLoadedAgents(
  cfg: Config,
  loadedAgents: Record<string, AgentConfig>,
  client: PluginInput["client"],
): Promise<void> {
  const result = new ConfigDraft(cfg).mergeAgents(loadedAgents)
  if (result.found === 0) {
    await log(client, "debug", "no Claude agents found under .claude/agents")
    return
  }
  await log(client, "info", "loaded Claude agents into OpenCode", {
    found: result.found,
    injected: result.injected,
    names: result.names,
  })
}

async function injectLoadedCommands(
  cfg: Config,
  loadedCommands: Record<string, CommandConfig>,
  client: PluginInput["client"],
): Promise<void> {
  const result = new ConfigDraft(cfg).mergeCommands(loadedCommands)
  if (result.found === 0) {
    await log(client, "debug", "no Claude commands found under .claude/commands")
    return
  }
  await log(client, "info", "loaded Claude commands into OpenCode", {
    found: result.found,
    injected: result.injected,
    names: result.names,
  })
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

      await injectLoadedAgents(cfg, loadedAgents, input.client)
      await injectLoadedCommands(cfg, loadedCommands, input.client)
    },
  }
}
