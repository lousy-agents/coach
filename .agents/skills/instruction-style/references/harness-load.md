# Harness load rules

Placement advice depends on **what triggers a file to load**, and the four
harnesses differ. Read this before moving a rule into a scoped or nested file.

These rules are version-sensitive. Treat the table as a starting hypothesis and
verify the trigger in the versions actually in use — a nested file that silently
never loads looks identical to one that works.

## The table

| Harness | Files it loads | What triggers a nested or scoped file | Confidence |
| --- | --- | --- | --- |
| **Claude Code** | `CLAUDE.md`, `CLAUDE.local.md`, `.claude/CLAUDE.md`, `.claude/rules/**` | Nested `CLAUDE.md` below the working directory loads when Claude reads a file in that directory. `.claude/rules/*` with `paths:` frontmatter loads when Claude reads a matching file. Both are lazy. `@import` is **eager** — an imported file loads at launch, so an import is not a context saving — and recurses at most **four hops** (current docs), so a deeper adapter chain silently drops its tail. | Verified against current docs (https://code.claude.com/docs/en/memory) |
| **Codex** | `AGENTS.override.md` then `AGENTS.md`, one file per directory | Walks project root **down to the working directory**, collecting one file per directory on that path, and concatenates with the nearest last. A file below the working directory is not on that path and does not load. Combined chain is capped at `project_doc_max_bytes`, 32 KiB by default. | Path rule verified against current docs; byte cap from Codex configuration docs, not independently exercised |
| **OpenCode** | v2: `AGENTS.md` only — the global `~/.config/opencode/AGENTS.md`, then every `AGENTS.md` from the session Location upward. v1 also read `CLAUDE.md` as a fallback and loaded every glob in `opencode.json`'s `instructions` array. | Initial load is **upward** only. **v2 also injects nested files on read**: an `AGENTS.md` below the Location is skipped at start, and when the read tool reads a file or lists a directory OpenCode discovers `AGENTS.md` from that target upward to (not including) the Location and injects each once per session. `instructions[]` is parsed but **not loaded** on v2; it is the eager mechanism on v1 only. | Verified against current v2 docs (https://opencode.ai/v2/docs/instructions/); v1 behavior from older docs, not re-exercised |
| **GitHub Copilot CLI** | `.github/copilot-instructions.md`, `.github/instructions/**/*.instructions.md`, `AGENTS.md`, `CLAUDE.md`, `.claude/CLAUDE.md`, `GEMINI.md`, plus user-level `~/.copilot/copilot-instructions.md` and `~/.copilot/instructions/**/*.instructions.md` | Discovers instruction files at the repository root, the working directory, the directories between them, and any directory nested in the path of a file it is working on. `*.instructions.md` with an `applyTo` glob is the path-scoped mechanism. All applicable files are **combined**: identical duplicates are dropped and no general precedence is defined between them. `@path` references inside a core are expanded immediately and recursively. | Verified against current GitHub Copilot CLI docs (https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-custom-instructions) |

## OpenCode: check the installed major version

Current v2 documentation settles the nested question: a nested `AGENTS.md` is
injected on read or list, once per session. The upward-only description and
the feature requests that cited it describe v1. Do not describe the product as
an open question; describe the version in front of you.

- Run `opencode --version`. On **v2**, a nested `AGENTS.md` is a real OpenCode
  projection, and `instructions[]` in `opencode.json` is parsed but never
  reaches the model.
- On **v1**, nested files never load. `opencode.json`'s `instructions` array is
  the mechanism: it takes globs such as `packages/*/AGENTS.md` and loads them
  eagerly, at the cost of loading them always.
- Version **unknown**: put the rule in the nested `AGENTS.md` and list the same
  glob in `instructions[]`. v2 ignores the array and v1 ignores the nested
  file, so neither version sees the rule twice.

## What follows for placement

- Place a rule on the **intersection** of the load paths of the harnesses that
  need it. Access-triggered loading of a nested file exists on Claude Code
  (nested `CLAUDE.md`, `.claude/rules` `paths`), OpenCode v2 (nested
  `AGENTS.md`), and Copilot CLI (nested `AGENTS.md`, `applyTo`). Codex keys off
  the working directory, and sessions usually start at the repository root, so
  Codex is the harness a nested file silently misses.
- A nested `AGENTS.md` under a root-started session loads on OpenCode v2 and
  Copilot CLI, is inert on Codex, and is invisible to Claude Code — which reads
  `CLAUDE.md` — unless a nested `CLAUDE.md` beside it imports it. A rule that
  silently vanishes for one harness is still a projection, and worse than a
  long shared file, because the content vanishes rather than merely costing
  tokens. Record the gap.
- Prefer a shared `AGENTS.md`, or a document plus a one-line pointer from each
  core, when the harnesses disagree. Plain documents need no mechanism: every
  harness reads them with ordinary file tools when a task calls for them.
- Claude Code reads `CLAUDE.md`, not `AGENTS.md`. A repository standardizing on
  `AGENTS.md` needs a thin `CLAUDE.md` that imports it; that import is eager
  and recurses at most four hops.
- If `AGENTS.md` and `CLAUDE.md` are both full files rather than core plus thin
  adapter, Copilot CLI loads both, drops only byte-identical duplicates, and
  defines no precedence between them — so keep the adapter to the import line
  plus harness-private routing. Copilot CLI also expands the adapter's
  `@AGENTS.md` reference; whether that expanded copy is deduplicated is not
  documented, so a thin adapter may still cost the core's tokens twice there,
  without producing two conflicting rules.
- Treat a harness-private scoped file as a **projection**. Record which
  harnesses no longer see the rationale, and say so in the report.
