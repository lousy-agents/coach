import fs from "node:fs/promises"
import path from "node:path"

import { parseFrontmatter, parseTools } from "./claude-agents-frontmatter.ts"
import type { AgentConfig, PermissionAction } from "./claude-agents-types.ts"

const CLAUDE_TOOL_TO_PERM: Record<string, string> = {
  Read: "read",
  Grep: "grep",
  Glob: "glob",
  Bash: "bash",
  Edit: "edit",
  Write: "edit",
}

const PERM_KEYS = ["read", "edit", "glob", "grep", "list", "bash"] as const

export function permissionsFromTools(
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

export async function loadClaudeAgents(agentsDir: string): Promise<Record<string, AgentConfig>> {
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
