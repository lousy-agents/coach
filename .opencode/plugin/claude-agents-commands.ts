import fs from "node:fs/promises"
import path from "node:path"

import { parseFrontmatter } from "./claude-agents-frontmatter.ts"
import type { AgentConfig, CommandConfig, Config } from "./claude-agents-types.ts"

function commandName(data: Record<string, string>, filePath: string): string {
  const fromFm = data.name?.trim()
  if (fromFm) return fromFm
  return path.basename(filePath, path.extname(filePath))
}

export async function loadClaudeCommands(commandsDir: string): Promise<Record<string, CommandConfig>> {
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

export class ConfigDraft {
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
