/**
 * Loads Claude Code subagents and slash commands into OpenCode.
 * Canonical sources: .claude/agents/ and .claude/commands/ — do not mirror bodies.
 */
import path from "node:path"

import { loadClaudeAgents } from "./claude-agents-agents.ts"
import { ConfigDraft, loadClaudeCommands } from "./claude-agents-commands.ts"
import type { AgentConfig, CommandConfig, Config, PluginInput } from "./claude-agents-types.ts"

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
