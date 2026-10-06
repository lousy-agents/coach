# Opus 5.5 Prompt Effectiveness Rubric (Claude Code)

A scoring rubric for an LLM-as-judge step in a prompt-optimization loop. It scores a **prompt** (not a model output) on how well it fits Claude Opus 5.5 running inside Claude Code. Weights reflect documented impact on outcome quality, token use and turn count. Anchors and examples are calibrated for `task` prompts.

Grounding: Anthropic's *Prompting Claude Opus 5.5*, the Opus 5.5 pages it links to (What's new, the Opus 5 → 5.5 migration guide, Effort, Thinking, Refusals and fallback), *Prompting Claude Opus 5* (which the Opus 5.5 guide names as the starting point), and the cross-model *Prompting best practices* page, read 2026-10-05.

Tags: `[O55 § H]` is the Opus 5.5 guide under heading H, and `[O55 <page> § H]` is one of its linked Opus 5.5 pages. `[O5 § H]` is the Opus 5 guide. `[CC …]` is a Claude Code docs page. `[gen § H]` is cross-model guidance. `[prior]` is the author's judgment where no source speaks. API and harness settings are not scored. Source ledger, harness notes and Sonnet 5 differences are kept for maintainers in `../provenance/opus-5-5-rubric-provenance.md`; judging does not need them.

---

## 1. What the judge is scoring

**Prompt types** (declare one; some criteria are N/A for some types):

| Type | Where it lives | Notes |
|---|---|---|
| `task` | The user turn that starts a Claude Code session, or a `/command` body | Primary target. Calibrated here. |
| `claude-md` | `CLAUDE.md` / project instructions | Standing instructions; C3 and C9 weigh more. |
| `skill` | `SKILL.md` body | Procedural; C5 and C9 weigh more. |
| `subagent` | An agent definition's system prompt | C2 and C4 weigh more; often run at lower effort. |

**Runtime facts the judge assumes unless told otherwise:**

- Model is Claude Opus 5.5. Adaptive thinking is always on and cannot be disabled [O55 What's new § Thinking can't be disabled]. Effort defaults to `medium` in Claude Code and on the API [O55 § Calibrate effort] [CC model-config]; when effort is undisclosed, score C4 against `medium` and record `effort: undisclosed`.
- Assistant prefill, non-default sampling, `budget_tokens`, disabled thinking, and forced `tool_choice` are rejected with a 400 [O55 Thinking § Limits and feature compatibility] [O55 What's new § Breaking changes].
- Safety classifiers cover cybersecurity, biology and reasoning extraction; a decline is `stop_reason: "refusal"` [O55 § Safeguard refusals].
- When the profiled harness is Claude Code, it loads `CLAUDE.md` automatically and provides file read/edit, shell, search, web fetch and subagent tools. Context window: `undisclosed` unless the model ID carries a context-window suffix.
- Tokenizer ratio versus earlier models: `undisclosed`. Whether the user sees tool output: `undisclosed`.
- If the profiled harness is not Claude Code, judge G3 against that harness, and read `CLAUDE.md` throughout this rubric as that harness's project instruction file (for example `AGENTS.md`). In a harness with no repository or workspace (direct chat, including claude.ai with web or code-execution tools), C5 is N/A and C6 accepts an observable result in place of a command. If the harness is undisclosed, judge G3 capability-neutral and mark C5 N/A.

---

## 2. Scoring mechanics

- Each criterion scores **0–3** against its anchors. No half points; pick the anchor the evidence best matches.
- Conditional criteria (C10–C15) are scored only when their trigger applies; otherwise mark `N/A` and exclude the weight.
- **Aggregate** = `100 × Σ(weight × score) / Σ(weight × 3)` over applicable criteria.
- A failed **hard gate** (Section 3) caps the aggregate at **40** and sets `verdict: "blocked"`.
- Quote **evidence** (verbatim span, or "absent") for every score, and give **one fix** for every criterion scoring below 3. A fix is an edit to the prompt, not advice to the user.

**Verdict bands** (after gates):

| Aggregate | Verdict | Loop action |
|---|---|---|
| ≥ 85 and no criterion ≤ 1 | `pass` | Fit for Opus 5.5; fix any remaining problem per the stop rule. |
| 70 ≤ aggregate < 85, or ≥ 85 with a criterion ≤ 1 | `revise` | Fix every problem (stop rule); the top-3 ranking orders the edits. |
| aggregate < 70 | `rework` | Structural rewrite using the skill's prompt shape, then re-judge. |
| any, gate failed | `blocked` | Fix the gate first. |

**Anti-oscillation rule:** a fix that adds a verification step, a reasoning ritual ("think carefully"), or a generic prohibition is invalid unless the judge confirms it does not lower C3, C4, C6 or C8. Opus 5.5 already verifies and self-corrects [O5 § Self-correction], so additions of this kind usually trade one failure for another.

---

## 3. Hard gates

| Gate | Fails when the prompt… | Why |
|---|---|---|
| **G1 Rejected-control dependency** | relies on a pre-seeded assistant turn, a temperature/top_p setting, a thinking budget, "thinking off", or a forced tool choice to get its result | Each is a 400 on Opus 5.5. Replace with a direct instruction; to get a tool call, "say in the prompt when the tool applies". `[O55 What's new § Forced tool use is not supported]` `[O55 Thinking § Response prefill and forced tool use]` |
| **G2 Reasoning written into the response** | asks for the model's thinking or reasoning in the reply, verbatim or in a fixed format: a `<thinking>`/`<reasoning>` section, a reasoning field in JSON, a running log of its reasoning, "show your reasoning" | May be declined as `reasoning_extraction`, which has no fallback. A short explanation of the answer or a summary of actions taken is fine. `[O55 § Safeguard refusals]` `[O5 § Reasoning in the response]` |
| **G3 Invented capability** | names a tool, flag, permission, hook or syntax the harness does not expose, or assumes a capability the runtime profile doesn't confirm | An instruction the agent cannot execute produces a stall, a fabricated action, or a question back. `[gen § Tool usage]` `[prior]` |
| **G4 Authority conflict** | tries to override `CLAUDE.md`, system or safety instructions, or grants itself permissions those settings withhold ("you may force-push" where pushes need approval); stating boundaries within them is C7 | A user prompt cannot outrank them; the attempt wastes tokens. `[prior]` |

---

## 4. Criteria

Weights are relative. Conditional criteria drop out when N/A.

### Tier 1 — highest impact

#### C1. Complete specification and done condition — weight 15 `[O5 § Capability improvements]` `[gen § Be clear and direct]` `[gen § Add context to improve performance]`

**Why:** Opus 5 "performs best when given the complete task specification up front and left to run", and Opus 5.5 carries that over. Context and motivation let the model generalize correctly. A spec delivered piecemeal costs turns and quality.

| Score | Anchor |
|---|---|
| 3 | Observable end state; every deliverable named; repeated scope quantified ("all 11 files under `internal/api/`"); a "done when" the agent can check alone; the why and any decision already made are stated, or the one likely question is answered as an assumption. |
| 2 | End state and done condition clear, but the why is missing or one material decision is left for the agent to ask about. |
| 1 | Goal is an activity ("improve", "clean up", "look into") with no observable end state, or deliverables are ambiguous. |
| 0 | Intent must be guessed. |

**Fix pattern:** state the end state; enumerate or count the set; add a checkable done condition (under `Verification:` in the skill's prompt shape); add one sentence of why.

#### C2. Scope fidelity — weight 12 `[O5 § Task scope and over-verification]` `[gen § Overeagerness]`

**Why:** Opus 5 "can also expand the scope of a task, adding steps that weren't requested or applying its own judgment about what the task should be." The prompt should fix the scope and say how to handle a request that looks mistaken.

| Score | Anchor |
|---|---|
| 3 | Bounds the change (files, packages, behaviours); asks for the task at the intended scope; unrelated findings are reported, not fixed; a mistaken-looking request is flagged in a sentence and the work continues as asked. Or the task is narrow enough that expansion is unlikely and the prompt adds no scope lines. |
| 2 | Bounds stated, but missing the unrelated-findings line, the mistaken-request line, or both. |
| 1 | Silent on scope for a task where expansion is likely (refactor, migration, cleanup, "fix the bug"). |
| 0 | Invites open-ended improvement ("improve anything else you see fit"). |

**Fix pattern:** add "Change only X. List unrelated problems at the end rather than fixing them. If the request looks wrong, say so in a sentence and continue as asked."

#### C3. Inherited scaffolding removed — weight 12 `[O5 § Self-correction]` `[O5 § Task scope and over-verification]` `[O55 § Tools for complex visual inputs]` `[gen § Tool usage]`

**Why:** Opus 5 "catches and fixes its own mistakes well without prompting"; re-check instructions "add cost without improving results". Aggressive tool language ("CRITICAL: you MUST") over-triggers. Visual workarounds built for earlier models may no longer be needed.

| Score | Anchor |
|---|---|
| 3 | No re-check rituals, shouting modifiers, forced update cadence, anti-laziness prodding, persona without task effect, or stale workaround. |
| 2 | One low-cost inherited artifact (e.g. a mild "be thorough"). |
| 1 | Two or more artifacts, or one that materially changes behaviour ("after every 3 tool calls…", "ALWAYS use X"). |
| 0 | Dominated by scaffolding written for an older model. |

**Detection cues:** "double-check", "re-verify", "CRITICAL", "MUST", "if in doubt, use [tool]", "you are a senior…", "after every N steps".
**Fix pattern:** delete. Replace only if the behaviour regressed in your own evals.

#### C4. Effort and thinking fit — weight 10 `[O55 § Calibrate effort]` `[O55 § Thinking instructions in chat system prompts]` `[O55 § Prompts written for thinking disabled]` `[gen § Leverage thinking & interleaved thinking capabilities]`

**Why:** Effort, not prompt text, is "the main control for how much Claude Opus 5.5 thinks"; lowering it reduces thinking "more reliably than prompt instructions do". "Think carefully" lines can be removed with no clear quality loss. Rules telling the model not to think should go. A short general instruction beats a hand-scripted reasoning plan.

| Score | Anchor |
|---|---|
| 3 | Prompt complexity matches the session effort: at `low`/`medium` an ordered work list and narrow, concrete checks; at `high`+ decision criteria and edge cases. No thinking-volume instructions ("think carefully", "think step by step", "don't overthink", "don't think"). |
| 2 | Fits the effort but carries one thinking-volume line or one over-prescribed reasoning script. |
| 1 | Mismatched: an ambiguous multi-step task with no ordered work list at `low`, heavy reasoning scaffolding on a simple task, or two or more thinking-volume lines. |
| 0 | Hand-scripts the whole thought process, or forbids thinking. (Asking for reasoning in the reply is G2.) |

**Fix pattern:** delete thinking-volume lines; turn scripted reasoning into decision criteria. If the task needs more depth, note in `stop_reason` that effort should rise; do not lengthen the prompt for it.

### Tier 2 — strong impact

#### C5. Harness grounding; look before acting — weight 10 `[O55 § Explore context in multi-app workflows]` `[gen § Minimizing hallucinations in agentic coding]` `[gen § Reduce file creation in agentic coding]` `[prior]`

**Why:** Opus 5.5 "tends to get to work quickly, and on loosely specified tasks it helps to tell the model to look through the relevant sources before acting" (measured on multi-app tasks; applied to repositories by analogy). Read before claiming; clean up scratch files.

| Score | Anchor |
|---|---|
| 3 | Inspect local instructions, current state and named paths before changing anything; concrete paths/artifacts; only tools that exist; edit hygiene where relevant (targeted edits, remove temp files); does not restate `CLAUDE.md`. |
| 2 | Grounded, with one gap: no inspect-first step, or a vague target ("the config"). |
| 1 | Written for chat: asks for a description instead of the change, or names no targets. |
| 0 | Contradicts `CLAUDE.md` or depends on missing capability (also G3). |

#### C6. Verification as acceptance, not ritual — weight 8 `[O5 § Task scope and over-verification]` `[gen § Avoid focusing on passing tests and hardcoding]`

**Why:** Opus 5 "verifies its own work without being told to"; added verification steps "cause over-verification … removing them reduces wasted tokens with no loss in quality." The prompt's job is to define what passing means, not to add passes. Tests verify the solution; they don't define it.

| Score | Anchor |
|---|---|
| 3 | Done-check stated as an acceptance condition (a command, a grep, an observable result); where tests or golden files exist, says to report a wrong one rather than work around it; no added verification step. |
| 2 | Acceptance check generic ("tests pass"), or one extra verification step on top of a concrete check. |
| 1 | Verification is a ritual only ("make sure it works"), or absent where the change needs one. |
| 0 | Stacks verifier subagents or repeated verification passes, or invites passing tests by any means. |

#### C7. Autonomy boundaries and stop conditions — weight 8 `[O55 § Unattended agentic runs]` `[gen § Balancing autonomy and safety]`

**Why:** Opus 5.5 "is responsive to instructions that name the specific kinds of early stop you want it to avoid" and the stops you want, such as "when no work can advance without the user's input". Hard-to-reverse or shared actions need an explicit line.

| Score | Anchor |
|---|---|
| 3 | States what may be done without asking and what needs confirmation (push, delete, external posts) where the task touches them; names when to end the turn (done, or blocked on input only the user has); for unattended runs, names the early stops to avoid. |
| 2 | Boundaries inherited from `CLAUDE.md` and not contradicted; stop condition implicit in the done condition. |
| 1 | No stop condition, or no boundary on a task that touches shared or destructive surfaces. |
| 0 | Encourages bypassing safety checks or destructive shortcuts. |

#### C8. Output length and form, stated positively — weight 7 `[O5 § Response length and verbosity]` `[O5 § Written deliverable length]` `[O5 § User-facing progress updates]` `[gen § Control the format of responses]`

**Why:** Opus 5 replies and written files "run longer" than prior models', and effort does not reliably shorten them, so length is prompted for. "Positive examples … tend to be more effective than instructions about what not to do."

| Score | Anchor |
|---|---|
| 3 | Final message and any written file have a concrete target ("one line per file changed", "≤ 5 lines", a short example); stated as what to do; negatives only for hard limits. |
| 2 | Target present but vague ("keep it brief"), or one negative that could be a positive. |
| 1 | Negatives carry the steering ("don't be verbose", "no lists"), or no target where the output feeds a reader. |
| 0 | A list of prohibitions. |

#### C9. Context economy — weight 6 `[gen § Long context prompting]` `[gen § Structure prompts with XML tags]` `[prior]`

| Score | Anchor |
|---|---|
| 3 | No sentence removable without changing behaviour; long inputs precede the ask; distinct content wrapped in tags or labelled sections; no restated project instructions (a pointer such as "`CLAUDE.md` covers the test commands", or an instruction to read it, is not duplication). |
| 2 | Lean with minor redundancy, or one block that belongs in `CLAUDE.md`. |
| 1 | Noticeable padding, repetition, or the ask buried under long input. |
| 0 | Mostly filler, or duplicates `CLAUDE.md` at length. |

**Type note:** weight 12 for `claude-md` and `skill`.

### Tier 3 — conditional

#### C10. Delegation proportionality — weight 6 `[O5 § Controlling subagent spawning]` `[gen § Subagent orchestration]` — *trigger: the prompt mentions subagents or parallel agents, or spans several independent tracks*

**Why:** Opus 5 "delegates to subagents more readily than prior models"; delegation "multiplies cost and time when applied to small tasks."

| Score | Anchor |
|---|---|
| 3 | Delegates only genuinely independent, sizeable tracks; small or sequential work is done directly; no verifier subagents. |
| 2 | Delegation sensible but unbounded (no count or criterion). |
| 1 | Asks for subagents on work a few tool calls finish. |
| 0 | Asks for subagents to verify or double-check the main agent's work. |

#### C11. Review tasks: coverage before filtering — weight 8 `[O5 § Capability improvements]` `[O55 § Capabilities relevant to prompting]` — *trigger: code review, bug finding, audit*

**Why:** "If your review prompt says 'only report high-severity issues' or 'be conservative,' the model may follow that instruction literally and report less; ask it to report everything and filter in a separate pass instead." Opus 5.5 already raises fewer false alarms.

| Score | Anchor |
|---|---|
| 3 | Every finding with severity and confidence, plus a separate filter stage or a concrete inclusion bar. |
| 2 | Concrete bar without labels, or coverage without a bar. |
| 1 | Qualitative filter ("serious problems only", "be conservative"). |
| 0 | Tells the model to minimize findings or drop uncertain ones. |

#### C12. Frontend direction — weight 5 `[O55 § Frontend design defaults]` — *trigger: the prompt produces UI or visual design*

**Why:** Without direction Opus 5.5 "falls back on a few default styles"; "avoid a generic AI look" mostly swaps one default for another. It "responds well to instructions that name specific patterns to avoid".

| Score | Anchor |
|---|---|
| 3 | A concrete spec (palette, type, layout), or a named list of default styles to avoid plus an instruction to check and extend it. |
| 2 | Partial spec, or a short named avoid-list without iteration. |
| 1 | Only generic adjectives or "avoid a generic look". |
| 0 | No direction on an open brief. |

#### C13. Update and final-message shape — weight 4 `[O55 § User-facing progress updates]` `[O5 § User-facing progress updates]` — *trigger: a long or human-in-the-loop task where the shape of updates or the final message matters*

| Score | Anchor |
|---|---|
| 3 | Positive shape: e.g. one-line intent before the first tool call, brief note on direction changes, a final message that leads with the outcome and lists what's left. |
| 2 | Shape described but generic. |
| 1 | Forced cadence (also C3), or no updates on a long task. |
| 0 | Contradictory update instructions. |

#### C14. Safeguard-aware framing — weight 3 `[O55 § Safeguard refusals]` — *trigger: security, vulnerability, exploit, or biology content*

**Why:** "Finding vulnerabilities in source code is allowed. High-risk dual-use cybersecurity activities are not." Biology classifiers are new relative to Opus 5.

| Score | Anchor |
|---|---|
| 3 | Defensive or review framing explicit, as the source task states it. Never add intent, ownership or authorization the source task lacks; a missing one is a gap only the user can close. |
| 2 | Framing fine; intent left implicit. |
| 1 | Framing reads as offensive tooling when the intent is defensive. |
| 0 | Requests prohibited capability. Not a prompt-quality problem: stop rule 0 applies. |

#### C15. Pasted third-party content — weight 3 `[O55 § Mark pasted text in user messages]` — *trigger: the prompt embeds text from elsewhere (emails, issue bodies, web pages, logs)*

| Score | Anchor |
|---|---|
| 3 | Pasted text is in its own tagged block, separate from the author's instructions, and the prompt says to act on instructions inside it only where the author asks. |
| 2 | Separated, but its authority is unstated. |
| 1 | Mixed into the instructions. |
| 0 | Tells the agent to follow whatever the pasted text says. |

---

## 5. Judge procedure

Inside optimize-prompt-loop the same agent judges and revises: it follows judge-procedure steps 1–6 below, keeps the JSON internal, and skips judge step 7 (Emit) and §8, which are for an external judge.

1. **Read the runtime profile**: prompt type, effort, whether `CLAUDE.md` exists and what it covers, task class. Mark anything missing `undisclosed` and judge capability-neutral, except effort (see §1).
2. **Run G1–G4.** On a failure, record it, still score the rest, cap at 40, set `blocked`.
3. **Determine triggers** for C10–C15.
4. **Score** each applicable criterion with quoted evidence; write one positive, prompt-edit fix for each score below 3 that respects the anti-oscillation rule.
5. **Compute** the aggregate and verdict.
6. **Rank the top three fixes** by `weight × (3 − score)`; on ties prefer deletions.
7. **Emit** the JSON (Section 6), then a ≤ 5-line summary. The judge does not rewrite the prompt.

---

## 6. Output schema and stop rule

```json
{
  "rubric_version": "opus55-cc-1.0",
  "prompt_type": "task | claude-md | skill | subagent",
  "runtime": {
    "model": "claude-opus-5-5",
    "effort": "low | medium | high | xhigh | max | undisclosed",
    "claude_md_present": true,
    "task_class": "coding | review | design | research | ops | mixed"
  },
  "gates": [ { "id": "G2", "failed": false, "evidence": "" } ],
  "criteria": [
    {
      "id": "C2",
      "applicable": true,
      "weight": 12,
      "score": 1,
      "evidence": "\"migrate the logging\" — no bound on files; nothing on unrelated findings",
      "fix": "Add: \"Change only files under internal/api/. List unrelated problems at the end rather than fixing them.\""
    }
  ],
  "aggregate": 78.0,
  "verdict": "pass | revise | rework | blocked",
  "top_fixes": [ { "criterion": "C2", "impact": 24, "fix": "..." } ],
  "pass_number": 1,
  "stop": false,
  "stop_reason": "aggregate below pass band"
}
```

**Stop rule** (it replaces any other pass count for this rubric loop). A *problem* is a failed gate or an applicable criterion scoring below 3. Each pass judges the current draft, checks the stop conditions, and only then revises it to fix every problem a prompt edit can fix. Stop at the first of:

0. C14 scores 0: stop without revising or reframing, and report the request as outside policy in place of an optimized prompt;
1. no problems remain, setting aside any gap that only the user's answer can close (record those as assumptions or the one question);
2. a criterion a revision raised falls back after a later revision for another criterion (1 → 3 → 1): keep the shorter of the last two judged drafts and report its verdict. A score change on unchanged evidence is re-scored, not a reversal;
3. five judged passes.

Otherwise return the last judged draft; the final verdict is its band. When (3) ends the loop with problems left, report that verdict rather than continuing; a gate still failing reports `blocked`.

---

## 7. Loop integration notes (external loops)

- **Pass the previous result to the judge** to compute the delta and catch reversals.
- **Deletions first.** Most inherited prompts improve by subtraction (C3, C4, C6). Try the deletion-only fix before an additive one.
- **Effort is an input, not a fix.** If C4 keeps failing, the move may be raising the session's effort; say so in `stop_reason`.
- **Judge model.** Opus 5.5 can judge its own prompts; supply this rubric to any judge.
- **Ground truth beats the rubric.** Weights are an impact ordering, not a measured regression. Retune against your own evals and bump `rubric_version`.

---

## 8. Judge system prompt (external judges only)

```
You are scoring a prompt intended for Claude Opus 5.5 running in Claude Code. Score the prompt, not any output you would produce from it. Apply the attached rubric exactly: run the four hard gates, mark conditional criteria N/A when their trigger is absent, score each applicable criterion 0-3 against its anchors, quote verbatim evidence or write "absent", and write one positive, prompt-edit fix for every criterion scoring below 3. Prefer fixes that delete text. Compute the weighted aggregate, the verdict band, and the top three fixes ranked by weight x (3 - score). If a previous result is provided, report the delta, flag any criterion whose score reversed direction, and apply the stop rule. Return the JSON first, then a summary of at most five lines. Do not rewrite the prompt.
```

---

## 9. Calibration examples (`task`, effort `medium`, `CLAUDE.md` present)

**Example A — inherited prompt; fails Opus 5.5-specific C4 (expect `rework`)**

> You are an expert Go engineer. CRITICAL: you MUST think carefully and deeply before every edit. Clean up the logging in `internal/api/`. Think step by step through each file. When you're done, spawn two subagents to double-check your changes, then run a final verification pass yourself.

- G1–G4 pass. C10 triggered (subagents); C11–C15 N/A.
- C1 = 1 ("clean up" is an activity; no end state or done condition). C2 = 2 (a directory bound, nothing on unrelated findings). C3 = 0 (persona, CRITICAL/MUST, double-check ritual). C4 = 1 (two thinking-volume lines; `[O55 § Calibrate effort]`). C5 = 2 (path named, no inspect-first step). C6 = 0 (verifier subagents plus a verification pass). C7 = 1 (no stop condition). C8 = 1 (no output target). C9 = 1 (persona and thinking lines are padding). C10 = 0.
- Aggregate ≈ 32 → `rework`. Top fixes: C3 and C6 (delete the persona, CRITICAL line and both verification sentences), C1 (end state and done condition). Most of the fix is deletion.

**Example B — fitted prompt (expect `pass`)**

> Objective: every HTTP handler under `internal/api/` (11 files) logs through `log/slog` instead of the deprecated `logx` package, so the platform team can turn on JSON log shipping next sprint.
> Context: `CLAUDE.md` covers test and lint commands. `internal/logging/slog.go` already builds the shared logger.
> Constraints: change only files under `internal/api/`; keep log levels and message text as they are; if a `logx` call has no direct `slog` equivalent, use the closest level and list it in your summary. If the plan looks wrong, say so in a sentence and continue as asked.
> Work: read `CLAUDE.md` and `internal/logging/slog.go` first; make targeted edits; list unrelated problems at the end rather than fixing them. Commit on the current branch and leave pushing to me. End the turn when done, or when a decision only I can make blocks you.
> Verification: `go test ./internal/api/...` passes and `grep -rn logx internal/api` returns nothing. If a test is wrong, say so instead of working around it.
> Output: one line per file changed, then any calls you had to map by judgment and any unrelated problems.

- G1–G4 pass; C10–C15 N/A.
- C1–C9 = 3 (C4: ordered work list and one narrow check suit `medium`, no thinking-volume lines). Aggregate 100 → `pass`. A judge giving 2 on C8 or C9 still lands ≥ 95.

**Example C — borderline (expect `revise`)**

> Objective: `pkg/report/render.go` renders the summary table with aligned columns for every row width; today rows longer than 40 characters break alignment (issue #212).
> Context: `CLAUDE.md` covers commands. The golden files in `pkg/report/testdata/` define the expected output.
> Work: read `CLAUDE.md`, `render.go` and the golden files first; fix the alignment; while you're in there, improve anything else in the package you think could be better.
> Verification: `go test ./pkg/report/...` passes with a new golden case for a 60-character row. If a golden file is wrong, say so instead of regenerating it.
> Output: what changed and why, in at most five lines.

- G1–G4 pass; C10–C15 N/A.
- C1 = 3 (a judge reading the open invitation as an unnamed deliverable may give 2; the verdict does not change). C2 = 0 ("improve anything else … you think could be better" invites open-ended work). C7 = 2 (no stop line; stop implicit in the done condition). All others = 3.
- Aggregate ≈ 83 → `revise` (≈ 78 with C1 at 2). With C2 at 1 it is ≈ 88, but a criterion ≤ 1 still gives `revise`. Replacing the "while you're in there" clause with C2's fix pattern (both sentences) fixes C2.

---

## 10. Provenance and caveats

Sources were read 2026-10-05; every tag is mapped in the maintainer ledger. Weights are an impact ordering, not a measured regression; re-validate against your own evals.