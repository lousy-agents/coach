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

async function loadPluginFactory() {
  const pluginModule = await import(new URL("./claude-agents.ts", import.meta.url).href)
  return pluginModule.default
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

  const pluginFactory = await loadPluginFactory()
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

  const pluginFactory = await loadPluginFactory()
  const plugin = await pluginFactory({ directory: base, worktree: "" })

  const cfg: { agent?: Record<string, { steps?: number }> } = {}
  await plugin.config(cfg)

  await rm(base, { recursive: true, force: true })

  assert.strictEqual(cfg.agent?.camel?.steps, 10)
  assert.strictEqual(cfg.agent?.snake?.steps, 20)
  assert.strictEqual(cfg.agent?.kebab?.steps, 30)
})

test("loads real .claude/agents/*.md with expected structure", async (t) => {
  const repoRoot = path.resolve(path.join(import.meta.dirname, "..", ".."))
  const pluginFactory = await loadPluginFactory()
  const plugin = await pluginFactory({ directory: repoRoot, worktree: repoRoot })

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
    "ux-advocate",
    "workflow-integration-reviewer",
  ])

  for (const [name, agent] of Object.entries(cfg.agent ?? {})) {
    assert.ok(agent.description && agent.description.length > 0, `${name}: missing description`)
    assert.strictEqual(agent.mode, "subagent", `${name}: mode should be subagent`)
    assert.ok(agent.prompt && agent.prompt.length > 0, `${name}: missing prompt body`)
    assert.ok(agent.permission, `${name}: missing permission`)
    for (const permKey of ["read", "edit", "glob", "grep", "list", "bash", "todowrite"]) {
      assert.ok(Object.prototype.hasOwnProperty.call(agent.permission!, permKey), `${name}: missing ${permKey} permission`)
    }
    assert.strictEqual(
      agent.permission!.write,
      undefined,
      `${name}: write should not be a separate permission key -- the Write tool is covered by 'edit'`,
    )
  }

  await t.test("task-implementer keeps bash allowed -- git ownership is enforced by prose and branch protection, not tool gating", () => {
    assert.strictEqual(cfg.agent!["task-implementer"]?.permission?.bash, "allow")
    assert.strictEqual(cfg.agent!["task-implementer"]?.steps, 30)
  })

  await t.test("task-reviewer denies edit but keeps bash allowed for its read-only checks", () => {
    assert.strictEqual(cfg.agent!["task-reviewer"]?.permission?.edit, "deny")
    assert.strictEqual(cfg.agent!["task-reviewer"]?.permission?.write, undefined)
    assert.strictEqual(cfg.agent!["task-reviewer"]?.permission?.bash, "allow")
  })

  await t.test("ux-advocate host peer map names Coach roster seats", () => {
    const prompt = cfg.agent!["ux-advocate"]?.prompt ?? ""
    for (const name of ["product-sme", "system-design-expert", "spec-review-agent"]) {
      assert.ok(prompt.includes(name), `ux-advocate prompt missing ${name}`)
    }
  })

  await t.test("ux-advocate prompt inlines the published charter", async () => {
    const prompt = cfg.agent!["ux-advocate"]?.prompt ?? ""
    const charter = await fs.readFile(
      path.join(repoRoot, ".agents/skills/designing-for-intent/agents/ux-advocate.md"),
      "utf8",
    )
    const marker = "You are this product's UX and product-design advocate"
    const start = charter.indexOf(marker)
    assert.ok(start >= 0, "published charter missing expected body")
    const body = charter.slice(start).trim()
    assert.ok(prompt.includes(body), "ux-advocate prompt missing published charter body")
    for (const heading of [
      "Intent map --",
      "Interaction model --",
      "Orchestration surface --",
      "Agency risks --",
      "Open questions --",
      "Out of scope --",
    ]) {
      assert.ok(prompt.includes(heading), `ux-advocate prompt missing output-contract heading ${heading}`)
    }
  })
})

test("prompt body preserves leading and trailing blank lines", async () => {
  const { base, agentsDir } = await makeAgentsDir()
  const body = "\n\nFirst line.\n\n"
  await fs.writeFile(path.join(agentsDir, "spaces.md"), agentFile("spaces", body))

  const pluginFactory = await loadPluginFactory()
  const plugin = await pluginFactory({ directory: base, worktree: "" })

  const cfg: { agent?: Record<string, { prompt?: string }> } = {}
  await plugin.config(cfg)

  await rm(base, { recursive: true, force: true })

  assert.strictEqual(cfg.agent?.spaces?.prompt, body)
})

async function makeCommandsDir() {
  const base = await mkdtemp(path.join(os.tmpdir(), "claude-commands-"))
  const commandsDir = path.join(base, ".claude", "commands")
  await fs.mkdir(commandsDir, { recursive: true })
  return { base, commandsDir }
}

function commandFile(name: string, description: string, body: string): string {
  return `---\nname: ${name}\ndescription: ${description}\nargument-hint: [issue-number]\nmodel: inherit\n---\n${body}`
}

test("loads real .claude/commands/implement-issue.md", async () => {
  const repoRoot = path.resolve(path.join(import.meta.dirname, "..", ".."))
  const pluginFactory = await loadPluginFactory()
  const plugin = await pluginFactory({ directory: repoRoot, worktree: repoRoot })

  const cfg: {
    command?: Record<string, { description?: string; template?: string; subtask?: boolean }>
  } = {}
  await plugin.config(cfg)

  const cmd = cfg.command?.["implement-issue"]
  assert.ok(cmd, "implement-issue command missing")
  assert.ok(cmd.description && cmd.description.length > 0, "missing description")
  assert.ok(cmd.template && cmd.template.length > 0, "missing template")
  assert.match(cmd.template, /task-implementer/)
  assert.match(cmd.template, /task-reviewer/)
  assert.match(cmd.template, /orchestrator|delegate/i)
  assert.strictEqual(cmd.subtask, undefined, "implement-issue must not set subtask: true")
})

test("explicit pre-existing command is not overwritten", async () => {
  const { base, commandsDir } = await makeCommandsDir()
  await fs.writeFile(
    path.join(commandsDir, "implement-issue.md"),
    commandFile("implement-issue", "from claude", "Claude body with task-implementer"),
  )

  const pluginFactory = await loadPluginFactory()
  const plugin = await pluginFactory({ directory: base, worktree: "" })

  const explicit = {
    description: "explicit opencode",
    template: "Explicit template wins",
  }
  const cfg: {
    command?: Record<string, { description?: string; template?: string }>
  } = {
    command: { "implement-issue": explicit },
  }
  await plugin.config(cfg)

  await rm(base, { recursive: true, force: true })

  assert.strictEqual(cfg.command?.["implement-issue"]?.description, "explicit opencode")
  assert.strictEqual(cfg.command?.["implement-issue"]?.template, "Explicit template wins")
})

test("command body preserves leading and trailing blank lines", async () => {
  const { base, commandsDir } = await makeCommandsDir()
  const body = "\n\nOrchestrate $1.\n\n"
  await fs.writeFile(
    path.join(commandsDir, "spaces-cmd.md"),
    commandFile("spaces-cmd", "d", body),
  )

  const pluginFactory = await loadPluginFactory()
  const plugin = await pluginFactory({ directory: base, worktree: "" })

  const cfg: { command?: Record<string, { template?: string }> } = {}
  await plugin.config(cfg)

  await rm(base, { recursive: true, force: true })

  assert.strictEqual(cfg.command?.["spaces-cmd"]?.template, body)
})

test("a command delegating to a Workflow still carries its plan inline, since OpenCode has no Workflow tool to run it", async () => {
  const repoRoot = path.resolve(path.join(import.meta.dirname, "..", ".."))
  const pluginFactory = await loadPluginFactory()
  const plugin = await pluginFactory({ directory: repoRoot, worktree: repoRoot })

  const cfg: { command?: Record<string, { template?: string }> } = {}
  await plugin.config(cfg)

  const commands = Object.entries(cfg.command ?? {})
  assert.ok(commands.length > 0, "no commands loaded -- the assertions below would be vacuous")

  for (const [name, command] of commands) {
    const template = command.template ?? ""
    if (!/\bWorkflow\b|implement-issue-plan/.test(template)) continue

    for (const field of ["dependsOn", "criteriaIds", "acceptanceTest"]) {
      assert.ok(
        template.includes(field),
        `${name}: delegates planning to a workflow but never states the plan's ${field} field, ` +
          `so a harness without a Workflow tool has no contract to work from`,
      )
    }

    assert.match(
      template,
      /without a Workflow tool/i,
      `${name}: must say what to do in a harness that has no Workflow tool`,
    )
  }
})
