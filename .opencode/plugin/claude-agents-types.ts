export type PermissionAction = "allow" | "ask" | "deny"

export type AgentConfig = {
  description?: string
  mode?: "subagent" | "primary" | "all"
  prompt?: string
  steps?: number
  permission?: Record<string, PermissionAction | Record<string, PermissionAction>>
  [key: string]: unknown
}

export type CommandConfig = {
  description?: string
  template?: string
  agent?: string
  model?: string
  subtask?: boolean
  [key: string]: unknown
}

export type Config = {
  agent?: Record<string, AgentConfig>
  command?: Record<string, CommandConfig>
  [key: string]: unknown
}

export type PluginInput = {
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
