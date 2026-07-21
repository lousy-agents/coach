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

test("maxTurns supports camelCase, snake_case, and kebab-case", async () => {
  const { base, agentsDir } = await makeAgentsDir()
  await fs.writeFile(
    path.join(agentsDir, "camel.md"),
    "---\nname: camel\ndescription: d\nmaxTurns: 10\n---\nBody",
  )
  await fs.writeFile(
    path.join(agentsDir, "snake.md"),
    "---\nname: snake\ndescription: d\nmax_turns: 20\n---\nBody",
  )
  await fs.writeFile(
    path.join(agentsDir, "kebab.md"),
    "---\nname: kebab\ndescription: d\nmax-turns: 30\n---\nBody",
  )

  const pluginModule = await import(new URL("./claude-agents.ts", import.meta.url).href)
  const plugin = await pluginModule.default({ directory: base, worktree: "" })

  const cfg: { agent?: Record<string, { steps?: number }> } = {}
  await plugin.config(cfg)

  await rm(base, { recursive: true, force: true })

  assert.strictEqual(cfg.agent?.camel?.steps, 10)
  assert.strictEqual(cfg.agent?.snake?.steps, 20)
  assert.strictEqual(cfg.agent?.kebab?.steps, 30)
})

test("loads real .claude/agents/*.md with expected structure", async () => {
  const repoRoot = path.resolve(path.join(import.meta.dirname, "..", ".."))
  const pluginModule = await import(new URL("./claude-agents.ts", import.meta.url).href)
  const plugin = await pluginModule.default({ directory: repoRoot, worktree: repoRoot })

  const cfg: {
    agent?: Record<
      string,
      { description?: string; mode?: string; prompt?: string; steps?: number; permission?: Record<string, unknown> }
    >
  } = {}
  await plugin.config(cfg)

  const names = Object.keys(cfg.agent ?? {})
  assert.deepStrictEqual(names, [
    "epic-reviewer",
    "product-sme",
    "spec-review-agent",
    "system-design-expert",
    "task-implementer",
    "task-reviewer",
  ])

  for (const [name, agent] of Object.entries(cfg.agent ?? {})) {
    assert.ok(agent.description && agent.description.length > 0, `${name}: missing description`)
    assert.strictEqual(agent.mode, "subagent", `${name}: mode should be subagent`)
    assert.ok(agent.prompt && agent.prompt.length > 0, `${name}: missing prompt body`)
    assert.ok(agent.permission, `${name}: missing permission`)
    for (const permKey of ["read", "edit", "glob", "grep", "list", "bash", "todowrite"]) {
      assert.ok(Object.prototype.hasOwnProperty.call(agent.permission!, permKey), `${name}: missing ${permKey} permission`)
    }
    // 'write' is not modeled separately; Write tool is covered by the 'edit' permission.
    assert.strictEqual(agent.permission!.write, undefined, `${name}: write should not be a separate permission key`)
  }

  const implementerBash = cfg.agent!["task-implementer"]?.permission?.bash
  assert.ok(typeof implementerBash === "object" && implementerBash !== null, "task-implementer: bash permission should be scoped object")
  assert.strictEqual(implementerBash!["*"], "allow")
  assert.strictEqual(implementerBash!["git commit*"], "deny")
  assert.strictEqual(cfg.agent!["task-implementer"]?.steps, 30)
  // task-reviewer has no Edit tool, so edit is denied; write is not modeled separately.
  assert.strictEqual(cfg.agent!["task-reviewer"]?.permission?.edit, "deny")
  assert.strictEqual(cfg.agent!["task-reviewer"]?.permission?.write, undefined)
  assert.strictEqual(cfg.agent!["task-reviewer"]?.permission?.bash, "allow")
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
