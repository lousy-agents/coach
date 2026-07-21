import assert from "node:assert"
import fs from "node:fs/promises"
import { mkdtemp, rm } from "node:fs/promises"
import os from "node:os"
import path from "node:path"
import test from "node:test"

async function makeAgentsDir() {
  const base = await mkdtemp(path.join(os.tmpdir(), "claude-agents-"))
  const agentsDir = path.join(base, ".claude", "agents")
  await fs.mkdir(agentsDir, { recursive: true })
  return { base, agentsDir }
}

function agentFile(name: string, body: string): string {
  return `---\nname: ${name}\ndescription: test agent\n---\n${body}`
}

test("agent key order is deterministic regardless of fs.readdir order", async () => {
  const { base, agentsDir } = await makeAgentsDir()
  await fs.writeFile(path.join(agentsDir, "b-agent.md"), agentFile("b-agent", "B"))
  await fs.writeFile(path.join(agentsDir, "a-agent.md"), agentFile("a-agent", "A"))
  await fs.writeFile(path.join(agentsDir, "c-agent.md"), agentFile("c-agent", "C"))

  const original = fs.readdir
  fs.readdir = async (
    filePath: string | Buffer | URL,
    options?: { encoding?: BufferEncoding | null; withFileTypes?: boolean } | BufferEncoding | null,
  ): Promise<any> => {
    const entries = await original(filePath, options as any)
    if (Array.isArray(entries)) {
      return [...entries].reverse()
    }
    return entries
  }

  const pluginModule = await import(new URL("./claude-agents.ts", import.meta.url).href)
    const pluginFactory = pluginModule.default
    const plugin = await pluginFactory({ directory: base, worktree: "" })

    try {
      const cfg: { agent?: Record<string, { prompt?: string }> } = {}
      await plugin.config(cfg)

    const names = Object.keys(cfg.agent ?? {})
    assert.deepStrictEqual(names, ["a-agent", "b-agent", "c-agent"])
  } finally {
    fs.readdir = original
    await rm(base, { recursive: true, force: true })
  }
})

test("prompt body preserves leading and trailing blank lines", async () => {
  const { base, agentsDir } = await makeAgentsDir()
  const body = "\n\nFirst line.\n\n"
  await fs.writeFile(path.join(agentsDir, "spaces.md"), agentFile("spaces", body))

  const pluginModule = await import(new URL("./claude-agents.ts", import.meta.url).href)
  const plugin = await pluginModule.default({ directory: base, worktree: "" })

  const cfg: { agent?: Record<string, { prompt?: string }> } = {}
  await plugin.config(cfg)

  await rm(base, { recursive: true, force: true })

  assert.strictEqual(cfg.agent?.spaces?.prompt, body)
})
