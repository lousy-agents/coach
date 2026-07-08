# Skill Review Checklist

Detailed criteria for each review dimension. Referenced by the skill-reviewer procedure.

---

## Scoring Rubric

Score each dimension on a 0–2 scale:

- **0** = Missing or fundamentally broken — the skill will not be discovered, will not execute correctly, or actively misleads the agent
- **1** = Present but flawed — partially works, poor design, brittle, or would fail under non-trivial use
- **2** = Solid — correct, well-designed, an agent can reliably discover and execute this skill

A dimension score captures three layers of evaluation, applied in order:

| Layer | Question | If no → |
|-------|----------|---------|
| **Exists?** | Is the element present at all? | Score 0 |
| **Works?** | Does it function correctly for typical cases? | Score 1 |
| **Well-designed?** | Is it robust, maintainable, and effective under edge cases? | Score 2 |

---

## Frontmatter

| # | Check | Severity | Detail |
|---|-------|----------|--------|
| F1 | `name` field exists | ❌ Blocker | Required for discovery |
| F2 | `name` is 1–64 chars | ❌ Blocker | Will be silently ignored if too long |
| F3 | `name` uses only lowercase alphanumeric + hyphens | ❌ Blocker | No spaces, underscores, or uppercase |
| F4 | `name` matches folder name | ❌ Blocker | `skills/my-skill/` → `name: my-skill` |
| F5 | `description` field exists | ❌ Blocker | Primary discovery mechanism |
| F6 | `description` ≤ 1024 chars | ⚠️ Warning | Truncated beyond limit |
| F7 | No YAML syntax errors | ❌ Blocker | Unescaped colons, tabs, bad quoting |
| F8 | Frontmatter delimiters present | ❌ Blocker | Must have opening and closing `---` |
| F9 | `argument-hint` is concise if present | ⚠️ Warning | Shown in slash-command UI |
| F10 | Boolean fields are valid YAML booleans | ⚠️ Warning | `true`/`false`, not strings |

### What you're actually looking for

- Does the frontmatter parse without errors, or does the agent silently get no metadata?
- Is the `name` a meaningful slug a human would guess, or generic filler (`my-skill`, `helper`)?
- Would the `description` alone — without reading the body — be enough for an agent to decide "yes, this is the right skill for this task"?
- Test: paste the `description` into an agent prompt as context. Does it lead the agent toward the correct invocation?

### Cross-references

- F5/F6 directly gates D1–D6 — if `description` is missing or truncated, description quality checks are moot.
- F4 (name/folder mismatch) is a leading indicator for broader sloppiness — if this is wrong, expect other structural issues.

---

## Description Quality

| # | Check | Severity | Detail |
|---|-------|----------|--------|
| D1 | Explains what the skill does | ❌ Blocker | First clause should state the action |
| D2 | Contains trigger keywords | ❌ Blocker | Words a user would type that should invoke this |
| D3 | Includes "Use when" scenarios | ⚠️ Warning | Helps agent match intent to skill |
| D4 | Not vague or generic | ❌ Blocker | "A helpful skill" = undiscoverable |
| D5 | Covers synonyms/alternatives | ⚠️ Warning | "test" + "verify" + "check" + "validate" |
| D6 | Doesn't duplicate another skill's description | ⚠️ Warning | Causes ambiguous routing |

### What you're actually looking for

- **Discovery effectiveness:** Write 3 hypothetical user prompts that *should* trigger this skill. Does the description contain enough signal to match? If you have to squint to see the connection, the agent won't find it either.
- **Routing precision:** Write 2 prompts that should *not* trigger this skill. Does the description clearly exclude them, or is it so broad it would match everything? 
- **Action vs. identity:** Does the description say what the skill *does* (action), or what it *is* (identity)? "Generates unit tests for Python functions" is actionable. "A testing skill" is identity — undiscoverable.
- **Keyword density:** Are domain-specific terms present? An agent searching for "deploy" won't match a description that only says "release" unless both are included.

### Cross-references

- D4 (vague description) is the #1 predictor of a skill that never gets invoked, regardless of how good the body is. A low D4 score makes every other dimension irrelevant in practice.
- D2 + D5 together determine discoverability. D2 without D5 means the skill works for users who use the exact right word but fails for everyone else.

---

## Body Structure

| # | Check | Severity | Detail |
|---|-------|----------|--------|
| B1 | Has "When to Use" section | ⚠️ Warning | Concrete scenarios, not restated description |
| B2 | Has "Procedure" section | ❌ Blocker | Step-by-step instructions the agent can follow |
| B3 | Procedures are actionable | ❌ Blocker | Each step is a concrete action, not a vague suggestion |
| B4 | Procedures are self-contained | ⚠️ Warning | No assumed external knowledge |
| B5 | Uses relative paths for resources | ❌ Blocker | `./scripts/foo.sh`, not absolute paths |
| B6 | Under ~500 lines | ⚠️ Warning | Offload to `./references/` if larger |
| B7 | Clear heading hierarchy | ⚠️ Warning | H2 for sections, H3 for subsections |

### What you're actually looking for

- **Procedure fidelity test:** Could a junior developer follow the procedure steps to produce the same output as the skill intends? If steps require unstated expertise or judgment calls, they're not actionable — they're aspirational.
- **Completeness vs. bloat:** Are the procedure steps *sufficient* to complete the task, or is there a gap where the agent has to guess? Conversely, are there steps that add no value ("Step 1: Understand the requirements")?
- **Tool specificity:** Do procedure steps reference the *specific tools* the agent should use (e.g., "use `grep_search` to find..."), or do they say vague things like "search the codebase"? Agents perform better with tool-specific instructions.
- **Error paths:** Do procedures handle the case where an expected file doesn't exist, or a search returns nothing? Missing error paths cause agents to stall silently.

### Cross-references

- B2 + B3 are the execution backbone. A skill can score perfectly on frontmatter and description, but if procedures are missing or vague, the agent will be *found* but will *fail to execute*. This is worse than not being found — it wastes the user's time.
- B6 (size) directly impacts P2 (instructions stage budget). A 600-line SKILL.md that doesn't use references will blow the context budget.

---

## Progressive Loading

| # | Check | Severity | Detail |
|---|-------|----------|--------|
| P1 | Discovery stage is lean | ⚠️ Warning | `name` + `description` < ~100 tokens |
| P2 | Instructions stage is focused | ⚠️ Warning | Full SKILL.md body < ~5000 tokens |
| P3 | Resources are one level deep | ⚠️ Warning | Don't nest references within references |
| P4 | Large content is in reference files | ⚠️ Warning | Not inlined in SKILL.md body |

### What you're actually looking for

- **Budget discipline:** Progressive loading exists because context windows are finite. Is each stage optimized for its budget, or is the skill front-loading content the agent doesn't need yet?
- **Reference architecture:** When the skill *does* reference sub-files, are those files focused enough that reading one gives the agent exactly what it needs for the current step? Or does it load a 200-line reference just to use one paragraph?
- **Nesting depth:** References that reference other references create cascading reads. Each level costs latency and tokens. One level deep is fine; two is suspicious; three is a design failure.

### Cross-references

- P2 over budget + B6 over limit = the skill is definitely monolithic (confirms A2 anti-pattern).
- P1 over budget usually means the description is doing too much — trying to be both the discovery surface AND the instruction set. If P1 is high, expect D1–D4 to reveal an unfocused description.

---

## Resources

| # | Check | Severity | Detail |
|---|-------|----------|--------|
| R1 | Referenced files exist | ❌ Blocker | No broken `./scripts/` or `./references/` links |
| R2 | Scripts are documented | ⚠️ Warning | Usage, inputs, outputs noted |
| R3 | References are single-topic | ⚠️ Warning | One concern per file |
| R4 | No unreferenced files | ⚠️ Warning | Dead files add confusion |

### What you're actually looking for

- **Broken links are silent failures:** An agent that reads a reference path and gets nothing back will either hallucinate the content or skip the step entirely. Neither is acceptable. R1 is a blocker because the failure mode is invisible.
- **Dead files signal drift:** Unreferenced files (R4) mean the skill has evolved but its resources haven't been cleaned up. This predicts other staleness issues — are the *referenced* files still accurate?
- **Script safety:** If scripts exist, do they validate inputs? Could a malformed argument cause unintended side effects? Scripts execute with the user's permissions.

---

## Anti-Patterns

| # | Pattern | Severity | Fix |
|---|---------|----------|-----|
| A1 | Vague description | ❌ Blocker | Add specific keywords and "Use when" triggers |
| A2 | Monolithic SKILL.md | ⚠️ Warning | Extract to `./references/` and `./scripts/` |
| A3 | Name/folder mismatch | ❌ Blocker | Rename one to match the other |
| A4 | Missing procedures | ❌ Blocker | Add step-by-step Procedure section |
| A5 | Over-broad scope | ⚠️ Warning | Split into multiple focused skills |
| A6 | Unreferenced dependencies | ⚠️ Warning | Document required MCP servers, tools, or extensions |
| A7 | No "When to Use" section | ⚠️ Warning | Add concrete trigger scenarios |
| A8 | Hardcoded paths | ❌ Blocker | Use relative paths from SKILL.md |
| A9 | Duplicated content across skills | ⚠️ Warning | Extract shared content to a common reference |

---

## Skill Smells — Bad Examples

Concrete examples of what bad skills look like. Use these to calibrate scoring and quickly identify problems.

### Smell: The Undiscoverable Description

```yaml
---
name: helper
description: "A helpful skill for doing things."
---
```

**Why it fails:** Zero keywords, zero specificity. An agent searching for *any* task could match this — which means it will match *no* task reliably. The name `helper` is equally meaningless. This skill will never be invoked.

**Scoring:** D1=0, D2=0, D4=0. Blocker-level failure on discoverability.

### Smell: The Everything Skill

```yaml
---
name: project-manager
description: "Manages projects. Creates files, runs tests, deploys code, reviews PRs, writes documentation, sets up CI/CD, monitors performance, and handles incidents."
---
```

**Why it fails:** Eight unrelated responsibilities in one skill. The agent can't tell when to invoke it because it applies to nearly everything. Each responsibility deserves its own skill with focused procedures. The description is long but says nothing specific about any one capability.

**Scoring:** D4=1 (not vague per se, but unfocused), A5=⚠️. The skill *sounds* capable but will produce poor results because its procedures can't adequately cover eight domains.

### Smell: The Aspirational Procedure

```markdown
## Procedure

1. Understand the user's requirements
2. Analyze the codebase
3. Implement the best solution
4. Verify the changes work correctly
```

**Why it fails:** These aren't steps — they're wishes. An agent reading "analyze the codebase" has no idea which tool to use, what to search for, or what "analyze" means in this context. Every step requires unstated judgment. Compare this to: "Use `grep_search` to find all files matching `*.test.ts` in the `src/` directory."

**Scoring:** B2=1 (section exists but useless), B3=0 (no step is actionable).

### Smell: The Restated Description

```yaml
---
name: code-review
description: "Reviews code for quality issues."
---
```

```markdown
## When to Use

Use this skill when you need to review code for quality issues.

## Procedure

1. Review the code
2. Report quality issues
```

**Why it fails:** The body adds nothing that the description didn't already say. "When to Use" just restates the description. The procedure restates the description again. Three layers of the same vague sentence. No specificity, no tool references, no concrete actions at any level.

**Scoring:** B1=0 (restated, not concrete scenarios), B3=0, D4=0.

### Smell: The Context Bomb

A SKILL.md with 800+ lines, no `./references/` folder, multiple checklists, full API documentation, and three different code examples all inlined in the body.

**Why it fails:** Blows the ~5000 token instructions budget. The agent loads the entire thing into context even when it only needs the procedure steps. Progressive loading can't help because everything is in one file. The agent's performance degrades on the actual task because its context is consumed by the skill itself.

**Scoring:** P2=0, P4=0, B6=0, A2=⚠️.

### Smell: The Phantom Reference

```markdown
## Procedure

1. Run the validation script: `./scripts/validate.sh`
2. Check results against the reference: `./references/expected-output.md`
```

But the `scripts/` folder doesn't exist, and `expected-output.md` was renamed to `validation-criteria.md` three months ago.

**Why it fails:** The agent will try to read files that don't exist, get empty results, and either hallucinate what the script would have done or stall. Broken references are silent failures — no error message, just wrong behavior.

**Scoring:** R1=0 (blocker — broken links). Leading indicator: if references are broken, the procedures that depend on them are also broken (B3 is likely 0–1).

### Smell: The Hardcoded Path

```markdown
## Procedure

1. Read the config at `/Users/jsmith/projects/my-app/.github/skills/deploy/config.yaml`
2. Run `/usr/local/bin/deploy-tool --env production`
```

**Why it fails:** Absolute paths work on exactly one machine. Any other user, CI environment, or workspace layout will fail. The skill is not portable.

**Scoring:** B5=0, A8=❌.

### Smell: The Keyword-Stuffed Description

```yaml
---
name: test-runner
description: "test tests testing tester tested run runner running execute execution verify verification validate validation check checker checking assert assertion spec specification suite suites unit integration e2e end-to-end functional regression"
---
```

**Why it fails:** While D2 and D5 want keywords and synonyms, this is a keyword soup with no actionable information. An agent can't extract *what this skill does* from a bag of words. Discovery might match, but the agent has no basis to decide if this skill is appropriate for the task at hand.

**Scoring:** D1=0 (doesn't explain what it does), D2=2 (keywords present, technically), D4=0 (maximally vague despite being long).

---

## Good Skill Elements — Examples to Emulate

Concrete examples of well-crafted skill components. Use these as reference for what a score of 2 looks like.

### Good: Precise, Discoverable Description

```yaml
---
name: api-test-generator
description: >-
  Generate integration tests for REST API endpoints. Creates test files with
  request/response assertions using the project's existing test framework.
  Use when: adding tests for new endpoints, backfilling test coverage for
  existing routes, or scaffolding test suites after API changes. Supports
  Express, Fastify, and Koa. Keywords: test, spec, endpoint, route, HTTP,
  request, response, assert, coverage.
---
```

**Why it works:**
- First clause states the action ("Generate integration tests for REST API endpoints")
- "Use when" gives three concrete trigger scenarios
- Framework names aid discovery for users of specific stacks
- Keyword block covers synonyms without drowning out the description
- Under 1024 chars but rich enough for confident routing

### Good: Actionable, Tool-Specific Procedures

```markdown
## Procedure

### 1. Identify Target Endpoints

- Use `grep_search` with pattern `router\.(get|post|put|delete|patch)` in the `src/routes/` directory
- For each match, extract the HTTP method, path, and handler function name
- If no routes are found, check for `app.get`, `app.post` patterns (Express-style)

### 2. Analyze Request/Response Shapes

- For each handler, use `read_file` to read the handler implementation
- Identify: path parameters, query parameters, request body schema, response status codes, response body shape
- If TypeScript types exist for request/response, read those files for the canonical shape

### 3. Generate Test File

- Create one test file per route group (e.g., `users.test.ts` for all `/users/*` routes)
- Use the project's existing test runner (check `package.json` scripts for `jest`, `vitest`, or `mocha`)
- Each test should: set up test data, make the HTTP request, assert status code, assert response body shape
- Include at least one happy-path test and one error-path test (invalid input, not found) per endpoint
```

**Why it works:**
- Each step names the exact tool to use (`grep_search`, `read_file`)
- Fallback behavior is specified ("If no routes are found, check for...")
- Outputs are concrete ("Create one test file per route group")
- Error paths are covered ("one error-path test per endpoint")
- An agent can follow these steps mechanically and produce correct output

### Good: Focused "When to Use" with Positive and Negative Signals

```markdown
## When to Use

Use this skill when:
- A user asks to add tests for an API endpoint or route
- A PR adds new endpoints and lacks corresponding test files
- Test coverage reports show untested route handlers
- A user says "scaffold tests", "add test coverage", or "write integration tests"

Do NOT use when:
- The user wants unit tests for a pure function (not API-related) — use `unit-test-generator` instead
- The user wants to run existing tests — use the terminal directly
- The user wants load/performance testing — this skill generates functional tests only
```

**Why it works:**
- Positive triggers give the agent confidence to match
- Negative triggers prevent mis-routing and point to the correct alternative skill
- Trigger phrases include exact words a user would type
- The boundary between this skill and adjacent skills is explicit

### Good: Well-Structured Reference Architecture

```
skills/
  api-test-generator/
    SKILL.md                          # ~200 lines: procedure + when-to-use
    references/
      test-patterns.md                # Common assertion patterns by framework
      supported-frameworks.md         # Detection logic for each framework
    scripts/
      detect-test-runner.sh           # Reads package.json, outputs runner name
```

**Why it works:**
- SKILL.md stays under budget — detail lives in references
- Each reference file is single-topic (R3)
- Scripts are focused utilities, not monolithic programs
- References are one level deep from SKILL.md (P3)
- Every file is referenced from SKILL.md — no dead files (R4)

### Good: Progressive Loading Discipline

```yaml
# Stage 1 — Discovery (~50 tokens)
name: api-test-generator
description: "Generate integration tests for REST API endpoints..."
```

```markdown
# Stage 2 — Instructions (~2000 tokens)
# SKILL.md body: When to Use + Procedure (5 steps, tool-specific)
# References mentioned but not loaded until needed
```

```markdown
# Stage 3 — Resources (on-demand, ~1500 tokens each)
# ./references/test-patterns.md — loaded only when Step 3 executes
# ./references/supported-frameworks.md — loaded only when Step 1 needs fallback detection
```

**Why it works:**
- Stage 1 is under 100 tokens — minimal cost when the skill is *considered* but not *selected*
- Stage 2 is under 5000 tokens — the agent gets complete instructions without context bloat
- Stage 3 is deferred — resources load only when the procedure step that needs them executes
- Total cost when fully loaded is reasonable; cost when *not* selected is near-zero

---

## Interpreting Review Results

Review scores tell a story. Individual dimensions matter, but patterns across dimensions reveal deeper issues.

### Leading Indicators

| Early Signal | What It Predicts |
|---|---|
| D4 = 0 (vague description) | Skill will never be invoked regardless of body quality. All other scores are moot. |
| F4 = 0 (name/folder mismatch) | Likely other structural mistakes — review with extra scrutiny. |
| B3 = 0 (procedures not actionable) | Skill will be found but will fail to execute — worse than not being found. |
| P2 = 0 (body over budget) | Agent performance degrades on the task because context is consumed by the skill itself. |
| R1 = 0 (broken references) | Every procedure step that depends on a reference is also broken. |

### Failure Patterns

**"Looks good, doesn't work"** — High D scores (description is discoverable), low B scores (procedures are vague). The skill gets selected but produces poor output. This is the most deceptive failure because the skill *appears* well-crafted at the discovery layer.

**"Works locally, fails everywhere"** — Hardcoded paths (A8), machine-specific scripts, or references to tools not documented as dependencies (A6). The skill works for the author but fails for anyone else.

**"Death by context"** — High P scores everywhere except P2 (body too large). The skill loads so much content that the agent's working memory for the actual task is starved. Common when authors inline reference material instead of extracting it.

**"Drift"** — R4 (unreferenced files) + R1 (broken references). The skill was good once but has evolved without maintaining its resource files. Procedures reference renamed or deleted files. The skill is decaying in place.

**"Identity crisis"** — D6 (duplicates another skill's description) + A5 (over-broad scope). The skill overlaps with other skills in the system, causing unpredictable routing. The agent might invoke this skill or its sibling depending on phrasing, producing inconsistent behavior.

### Cross-Reference Signals

Just as static analysis metrics can confirm or challenge a manual code review score, cross-referencing dimensions can reveal contradictions in a skill review:

| If you scored... | But also found... | Then revisit... |
|---|---|---|
| B3 = 2 (procedures actionable) | No tool names in any step | B3 — truly actionable procedures name their tools |
| D2 = 2 (trigger keywords present) | D1 = 0 (doesn't explain what it does) | D2 — keywords without context are keyword stuffing |
| P2 = 2 (body under budget) | B2 = 0 (no procedure section) | P2 — body is short because it's *empty*, not because it's efficient |
| R3 = 2 (references are single-topic) | R1 = 0 (references don't exist) | R3 — can't evaluate quality of nonexistent files |
| B4 = 2 (self-contained) | A6 = ⚠️ (unreferenced dependencies) | B4 — the procedures assume tools/extensions the skill doesn't mention |