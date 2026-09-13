# Discovering the prompt surface

Two methods. Either is acceptable; the tool is faster and more complete, the
globs need no prerequisite.

## Method 1 — doctor (optional, needs Node.js)

```sh
npx --yes @lousy-agents/cli doctor --format=json --ci > ./doctor-inventory.json
```

Write the file **at the worktree root**, not to `/tmp`, which is not writable in
every sandbox. Nothing ignores it by default, so deleting it is a required
Output step, not a courtesy: `rm ./doctor-inventory.json` once the inventory
and edges have been read.

If the permission layer denies the doctor command, or doctor fails, **say so in
the report and fall back to the globs below.** A denied doctor is not "doctor
unavailable, discovery complete"; it is discovery that has not happened yet.
Never report the surface as enumerated on a run that did not execute.

**Keep `--ci`, and ignore the exit code.** `--ci` means "force non-interactive
mode — skip elicitation prompts". Without it, in a TTY with no intent artifact,
doctor asks questions and an agent waits forever. The non-zero exit is
unrelated: it comes from blocking findings and fires with or without `--ci`.
Read the JSON; do not branch on the status.

Filter it; never read it whole. The output runs to tens of thousands of tokens,
and reading it raw costs more context than the globbing it replaced:

```sh
node -e 'const d=require("./doctor-inventory.json");for(const i of d.inventory)if(["instruction","subagent","agent","skill"].includes(i.constructType))console.log(i.harness,i.constructType,i.path)'
```

`skill` is in the filter because a skill's routing `description` is in scope.
Take only the path from those records and read the frontmatter description;
the procedure body below it is out of scope.

Fields: `inventory[]` carries `path`, `harness`, `constructType`
(`instruction`, `skill`, `subagent`, `agent`, `hook`, `mcp-server`) and
`loadMechanism` (`convention-loaded`, `referenced`). `edges[]` carries the
reference graph with `malformed` and `crossHarness` flags, so a `hard-import`
such as `CLAUDE.md → AGENTS.md` appears as an edge instead of something you have
to notice. Use `edges[].malformed` for the link check — resolving relative paths
by hand is error-prone, because a link resolves from its containing file's
directory and checking from the repository root reports every link in a
relocated page as dead.

Two limits worth knowing. Doctor's harness vocabulary is `claude`, `copilot`,
`codex`, `antigravity`, `hermes`, `crush`, `pi`, `shared` — there is **no
`opencode` value**, so OpenCode artifacts surface under `shared`, `claude`, or
`codex` paths and you find them by path, not by harness. And doctor is topology,
not evaluation: an empty `findings[]` is not a clean bill of health.

## Method 2 — fallback globs (no prerequisite)

Use these when Node or the network is unavailable, and say in the report that
discovery was manual and may be incomplete.

```
**/AGENTS.md
**/AGENTS.override.md
**/CLAUDE.md
CLAUDE.local.md
.claude/CLAUDE.md
.claude/rules/**
.claude/agents/**
.claude/commands/**
.claude/skills/**/SKILL.md
.github/copilot-instructions.md
.github/instructions/**/*.instructions.md
.github/agents/**/*.agent.md
.github/skills/**/SKILL.md
.agents/skills/**/SKILL.md
opencode.json{,c}
.opencode/opencode.json{,c}
.opencode/AGENTS.md
.opencode/agent{,s}/**
.opencode/command{,s}/**
.opencode/skill{,s}/**/SKILL.md
.codex/config.toml
.codex/agents/**
.codex/skills/**
```

`**/AGENTS.md` and `**/CLAUDE.md` recurse on purpose. A nested
`packages/*/AGENTS.md` is the scoped mechanism on Codex (working-directory
chain), OpenCode v2 (injected on read), and Copilot CLI (nested in the path of
a file being worked on), and a root-only glob misses it. Skip `node_modules/`
and vendored trees when expanding. The OpenCode entries are named rather than
`.opencode/**` because that directory also holds plugin `node_modules` and
other non-instruction files; the `{,s}` alternatives cover v1's singular
directory names and v2's plural ones.

## Lock surfaces — grep these before editing

Tests, hooks, and loaders match on instruction text verbatim, including its
formatting. Inventory what is locked **before the first edit**: a grep before
each deletion catches the phrase you thought to check and misses the one you
did not.

Search the test, hook, and loader sources for:

- **Headings and section titles** the code slices on.
- **Step markers** — a numbered step and any emphasis that follows it. Code that
  splits a file into sections by scanning for a step prefix makes that
  formatting load-bearing, so stripping the emphasis breaks the build.
- **Named commands and phrases** asserted as substrings.
- **Load-bearing frontmatter keys**: `applyTo`, `paths`, `excludeAgent`, and a
  Copilot agent file's `tools` and `model`. Stripping or rewriting these stops
  the file loading.
- **Glob values** in those keys. Expand each against the repository and confirm
  a non-empty match: a scoped file whose glob matches nothing is dormant and
  fails silently, looking maintained while never loading.
