---
name: skill-reviewer
description: 'Rigorously review, validate, and lint Agent Skills SKILL.md files (GitHub Copilot, Claude, Gemini CLI, Cursor, Codex, and any Agent Skills-compatible agent). Use when auditing, reviewing, checking, validating, debugging, fixing, or improving agent skills. Checks frontmatter, description quality, progressive loading, body structure, anti-patterns, and discoverability.'
argument-hint: 'Path to the skill folder or SKILL.md to review'
allowed-tools: Read, Grep, Glob
---

# Skill Reviewer

Performs a rigorous, structured review of agent skill definitions to ensure they are well-formed, discoverable, and effective.

## When to Use

- Reviewing a new or updated SKILL.md before merging
- Auditing existing skills for quality and correctness
- Validating that a skill follows best practices
- Improving a skill that isn't being discovered or invoked by agents

Do NOT use when:
- The user wants to create a new skill from scratch — help them author a new SKILL.md instead
- The user wants help writing .instructions.md or .prompt.md files — those are not SKILL.md
- The user wants to review non-skill agent configuration (copilot-instructions.md, AGENTS.md)

## Procedure

### 1. Locate the Skill

- If given a path, read the `SKILL.md` at that location
- If given a skill name, search for it under `skills/`, `.agents/skills/`, `.github/skills/`, or `.claude/skills/`
- Confirm the folder exists and contains a `SKILL.md`

### 2. Validate Frontmatter

Run every check from the [frontmatter checklist](./references/review-checklist.md#frontmatter). Key checks:

- `name` field exists, is 1–64 chars, lowercase alphanumeric + hyphens only
- `name` matches the containing folder name exactly
- `description` exists and is ≤ 1024 characters
- `description` contains keyword-rich trigger phrases (not vague like "a helpful skill")
- No YAML syntax errors (unescaped colons, tabs instead of spaces)
- Optional fields (`argument-hint`, `user-invocable`, `disable-model-invocation`) are valid if present

### 3. Evaluate Description Quality

The description is the **discovery surface** — it determines whether agents find and load the skill.

- Does it explain **what** the skill does?
- Does it explain **when** to use it with specific trigger words?
- Would an agent match it for the intended use cases?
- Are there synonyms or alternative phrasings for discoverability?
- Test: write 3 hypothetical user prompts that should trigger this skill — does the description match?

### 4. Review Body Structure

- Has a clear **When to Use** section with specific scenarios
- Has a step-by-step **Procedure** section
- Procedures are actionable and complete (an agent can follow them without external knowledge)
- References to sub-files use relative paths (`./scripts/`, `./references/`)
- Total body is under ~500 lines (offload to references if larger)

### 5. Check Progressive Loading

Skills load in three stages. Verify each is optimized:

| Stage | Budget | Check |
|-------|--------|-------|
| Discovery | ~100 tokens | `name` + `description` are concise and informative |
| Instructions | <5000 tokens | `SKILL.md` body is focused, not bloated |
| Resources | On-demand | Referenced files are one level deep from SKILL.md |

### 6. Inspect Resources

- Are `scripts/`, `references/`, and `assets/` folders used appropriately?
- Are scripts executable and documented?
- Are references focused (one topic per file)?
- Are file references in SKILL.md valid (no broken links)?

### 7. Detect Anti-Patterns

Flag any of these issues. See [anti-patterns list](./references/review-checklist.md#anti-patterns).

- Vague description that doesn't enable discovery
- Monolithic SKILL.md with everything inlined instead of using references
- Name/folder mismatch
- Missing procedures (description without actionable steps)
- Over-broad scope (skill tries to do too many unrelated things)
- Unreferenced dependencies (relies on tools/MCP servers not mentioned)
- No "When to Use" section

### 8. Produce Review Report

Output a structured review with:

```
## Skill Review: <skill-name>

### Summary
<one-line verdict: PASS / NEEDS WORK / FAIL>

### Scores
- Frontmatter: ✅ | ⚠️ | ❌
- Description Quality: ✅ | ⚠️ | ❌
- Body Structure: ✅ | ⚠️ | ❌
- Progressive Loading: ✅ | ⚠️ | ❌
- Resources: ✅ | ⚠️ | ❌
- Anti-Patterns: ✅ none detected | ⚠️ minor | ❌ major

### Issues
1. <issue + fix recommendation>

### Strengths
1. <what's done well>
```