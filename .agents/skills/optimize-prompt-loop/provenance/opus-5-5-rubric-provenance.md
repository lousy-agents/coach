# Provenance: Opus 5.5 prompt rubric (`opus55-cc-1.0`)

Maintainer record for `../references/opus-5-5-claude-code-prompt-rubric.md`. The skill never loads this file and a judge does not need it. It records where each gate and criterion came from, what was left out and why, the harness-level facts that are kept out of scoring, and where the rubric departs from the earlier Sonnet 5 rubric it was modelled on (`sonnet5-cc-1.0`, unpublished; §5 restates every criterion it compares, so that rubric is not needed to read this one).

Sources were read on 2026-10-05, in precedence order:

| # | Source | URL | Fetch |
|---|---|---|---|
| 1 | Prompting Claude Opus 5.5 (`O55`) | platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5-5.md | 200 |
| 2 | Prompting Claude Opus 5 (`O5`) | …/prompt-engineering/prompting-claude-opus-5.md | 200. The first request returned the best-practices page truncated at 32 KiB; the retry returned the correct page. |
| 3 | Prompting best practices (`gen`) | …/prompt-engineering/claude-prompting-best-practices.md | 200 |
| 4a | What's new in Claude Opus 5.5 | platform.claude.com/docs/en/models/opus-5-5/whats-new-opus-5-5.md | 200 |
| 4b | Migration guide, Opus 5 → 5.5 section | platform.claude.com/docs/en/models/opus-5-5/migration-guide.md | 200 |
| 4c | Effort | platform.claude.com/docs/en/build-with-claude/effort.md | 200 |
| 4d | Thinking (plus the "Steering thinking" page it links to) | platform.claude.com/docs/en/build-with-claude/thinking.md | 200 |
| 4e | Refusals and fallback | platform.claude.com/docs/en/build-with-claude/refusals-and-fallback.md | 200 |
| CC | Claude Code model configuration | code.claude.com/docs/en/model-config.md | 200 |

No URL needed the retry without `.md`. SDK samples and other models' guides were skipped.

Tag convention used in the rubric:
- `[O55 § H]` is the Opus 5.5 guide under heading H. `[O55 <page> § H]` is one of the pages in 4a–4e, which that guide links to for Opus 5.5.
- `[O5 § H]` is the Opus 5 guide, which the Opus 5.5 guide names as "a reasonable starting point".
- `[gen § H]` is the cross-model best-practices page.
- `[CC <page>]` is a Claude Code docs page (only `model-config` is cited).
- `[prior]` is the author's own knowledge, used only where no source speaks.

## 1. Heading ledger: Opus 5.5 guide

Every heading on the guide (the body has no H1, H3 or H4).

| Heading | Disposition | Evidence span |
|---|---|---|
| (intro, before first H2) | Rubric framing: Opus 5 guidance carries over. Grounds every `[O5]` tag (anti-oscillation rule, G2, C1–C3, C6, C8, C10, C11, C13). | "Existing Claude Opus 5 prompts should perform well without changes, and the patterns in Prompting Claude Opus 5 remain a reasonable starting point." |
| Capabilities relevant to prompting | C11 (code review recall); C3 (vision workarounds). The rest is capability description with no prompt lever, so it is excluded. | "Early testers also reported stronger code review, with more bugs caught than on Claude Opus 5 and fewer false alarms" |
| Calibrate effort | C4. `max_tokens`, effort choice and prompt-cache behaviour go to harness notes. | "To get less thinking, lower the effort level first. Lowering effort reduces thinking … more reliably than prompt instructions do." |
| Prompts written for thinking disabled | G2 (reasoning written into the response); C4 (no-thinking rules). Reading responses by block type is a harness note. | "remove the no-thinking rule either way" |
| Unattended agentic runs | C7 (named early stops and wanted stops). Continuation messages, checker models and system-prompt placement are harness notes. | "Claude Opus 5.5 is responsive to instructions that name the specific kinds of early stop you want it to avoid" |
| Safeguard refusals | G2; C14. Fallback handling is a harness note. | "Finding vulnerabilities in source code is allowed. High-risk dual-use cybersecurity activities are not." |
| User-facing progress updates | C13 (update shape). `thinking.display`, the message tool and turn-scoped reminders are harness notes. | "if you want more frequent or predictable updates, such as a one-line statement of intent before the first tool call and a short recap at the end, say so" |
| Explore context in multi-app workflows | C5 (look before acting). Measured on multi-app automation; applied to repository tasks by analogy, so C5 also carries `[prior]`. | "Claude Opus 5.5 tends to get to work quickly, and on loosely specified tasks it helps to tell the model to look through the relevant sources before acting." |
| Time signals for multiagent harnesses | Excluded from scoring. The elapsed-time line is harness-generated, and the one-sentence alternative belongs in a lead agent's system prompt, not a task prompt. Harness note. | "have your harness add a short line at the end of each message … `elapsed 340s / 1200s`" |
| Thinking instructions in chat system prompts | C4 ("think carefully" lines). The "treat that answer as done" sentence is chat-only, and the guide says to leave it out of agentic tasks, so it is not rewarded. | "consider removing them for Claude Opus 5.5. The model decides for itself how much to think" |
| Mark pasted text in user messages | C15 (separate pasted third-party text and scope its authority). ID generation is a harness note. | "mark which text is the user's own and which was pasted from somewhere else" |
| Tools for complex visual inputs | C3 (stale visual workarounds). Image resolution and crop tools or containers are harness notes. | "re-test whether you still need scaffolding you built for visual inputs on earlier models" |
| Frontend design defaults | C12. | "It responds well to instructions that name specific patterns to avoid" |

## 2. Gate and criterion sources

| ID | Name | Tags | Notes |
|---|---|---|---|
| §1 | Runtime facts | `[O55 What's new § Thinking can't be disabled]` `[O55 What's new § Breaking changes]` `[O55 Thinking § Limits and feature compatibility]` `[O55 § Calibrate effort]` `[O55 § Safeguard refusals]` `[CC model-config]` | The Thinking page's § Sampling parameters and § Response prefill and forced tool use sit under § Limits and feature compatibility. |
| G1 | Rejected-control dependency | `[O55 What's new § Forced tool use is not supported]` `[O55 Thinking § Response prefill and forced tool use]` | Scores the prompt's dependence on prefill, sampling, a thinking budget or disabled thinking, or forced tool choice. The fix is prompt text: "say in the prompt when the tool applies". |
| G2 | Reasoning written into the response | `[O55 § Safeguard refusals]` `[O5 § Reasoning in the response]` | Also supported by [O55 § Prompts written for thinking disabled] and [O55 Refusals § Keep reasoning in thinking blocks]. New relative to Sonnet 5. The `reasoning_extraction` category has no recommended fallback, so the prompt has to change. |
| G3 | Invented capability | `[gen § Tool usage]` `[prior]` | |
| G4 | Authority conflict | `[prior]` | No source states it. Kept from the Sonnet 5 rubric. |
| C1 | Complete specification and done condition | `[O5 § Capability improvements]` `[gen § Be clear and direct]` `[gen § Add context to improve performance]` | Folds in the Sonnet 5 rubric's C8 (front-loading). |
| C2 | Scope fidelity | `[O5 § Task scope and over-verification]` `[gen § Overeagerness]` | |
| C3 | Inherited scaffolding removed | `[O5 § Self-correction]` `[O5 § Task scope and over-verification]` `[O55 § Tools for complex visual inputs]` `[gen § Tool usage]` | |
| C4 | Effort and thinking fit | `[O55 § Calibrate effort]` `[O55 § Thinking instructions in chat system prompts]` `[O55 § Prompts written for thinking disabled]` `[gen § Leverage thinking & interleaved thinking capabilities]` | |
| C5 | Harness grounding; look before acting | `[O55 § Explore context in multi-app workflows]` `[gen § Minimizing hallucinations in agentic coding]` `[gen § Reduce file creation in agentic coding]` `[prior]` | |
| C6 | Verification as acceptance, not ritual | `[O5 § Task scope and over-verification]` `[gen § Avoid focusing on passing tests and hardcoding]` | |
| C7 | Autonomy boundaries and stop conditions | `[O55 § Unattended agentic runs]` `[gen § Balancing autonomy and safety]` | |
| C8 | Output length and form, stated positively | `[O5 § Response length and verbosity]` `[O5 § Written deliverable length]` `[O5 § User-facing progress updates]` `[gen § Control the format of responses]` | |
| C9 | Context economy | `[gen § Long context prompting]` `[gen § Structure prompts with XML tags]` `[prior]` | The Opus 5.5 tokenizer ratio is undisclosed (see §4). |
| C10 | Delegation proportionality | `[O5 § Controlling subagent spawning]` `[gen § Subagent orchestration]` | |
| C11 | Review tasks: coverage before filtering | `[O5 § Capability improvements]` `[O55 § Capabilities relevant to prompting]` | |
| C12 | Frontend direction | `[O55 § Frontend design defaults]` | |
| C13 | Update and final-message shape | `[O55 § User-facing progress updates]` `[O5 § User-facing progress updates]` | |
| C14 | Safeguard-aware framing | `[O55 § Safeguard refusals]` | |
| C15 | Pasted third-party content | `[O55 § Mark pasted text in user messages]` | |

## 3. Harness notes (not scored)

A prompt author cannot set any of these from a task prompt, so the rubric does not score them. Claude Code manages most of them.

- **Effort.** The API defaults to `medium` on Opus 5.5 (`high` on Opus 5) [O55 § Calibrate effort]. Claude Code also defaults Opus 5.5 to `medium`, set with `/effort` or `modelSettings` [CC model-config]. Raising effort is the fix when a task under-performs at `medium`; lengthening the prompt is not. Changing top-level effort mid-conversation invalidates the prompt cache [O55 § Calibrate effort].
- **Thinking.** It is always on and adaptive. `thinking: disabled` and `budget_tokens` return 400 [O55 What's new § Thinking can't be disabled]. In Claude Code, `ultrathink` is the only recognised keyword; "think hard" passes through as plain text [CC model-config].
- **`max_tokens`.** It covers thinking plus the reply. 128k worked well for agentic coding [O55 § Calibrate effort].
- **`thinking.display`.** The default is `omitted`. Progress notes between tool calls arrive as `thinking` blocks, and a client that renders only `text` blocks looks silent. `display: "updates"` (beta) restores them [O55 § User-facing progress updates] [O55 Migration § Text between tool calls is returned in thinking blocks].
- **Turn-scoped system messages.** A reminder ("The user hasn't heard from you in a while…") is appended after about five quiet tool steps, at most two or three times [O55 § User-facing progress updates]. It is harness-injected.
- **Unattended continuation.** Treat a text-only `end_turn` as a report. Send a continuation naming open items, or use a smaller checker model, and stop after two or three continuations [O55 § Unattended agentic runs]. The anti-early-stop paragraph goes at the end of the system prompt from the first request. Adding it later invalidates earlier thinking blocks.
- **Time budgets.** The harness appends `elapsed Ns / Ms` to each message. The budget is advisory [O55 § Time signals for multiagent harnesses].
- **Pasted-content IDs.** The application wraps pasted text in tags that share a random ID, and the system-prompt note explains them [O55 § Mark pasted text in user messages].
- **Visual inputs.** Higher resolution, crop tools, or a container with PIL and OpenCV help; they work better at higher effort [O55 § Tools for complex visual inputs].
- **Refusals and fallback.** A refusal is HTTP 200 with `stop_reason: "refusal"`. Server-side `fallbacks: "default"` does not retry `reasoning_extraction` [O55 Refusals § Server-side fallback]. In Claude Code on Opus 5.5, cyber-flagged requests re-run on Opus 4.8 and bio-flagged requests on Opus 5, keeping the session's effort level [CC model-config].
- **Forced tool use.** `tool_choice` `any` and `tool` return 400. Use `auto` with strict tools [O55 What's new § Forced tool use is not supported].
- **Append-only history.** Editing the system prompt, tools or earlier turns invalidates thinking blocks (400 on accounts created on or after 2026-08-31) [O55 What's new § Thinking blocks are tied to the model and the conversation].
- **Subagent caps.** `CLAUDE_CODE_MAX_SUBAGENT_SPAWN_DEPTH` and `CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS` [O5 § Controlling subagent spawning].

## 4. Runtime facts marked `undisclosed`

- **Tokenizer ratio versus Opus 5 or Sonnet 5.** None of the read sources states it; What's new defers to a model overview page that was outside the source list.
- **Whether Claude Code hides or collapses tool output from the user on Opus 5.5.** The docs describe truncation of large outputs, not user visibility.
- **Whether a Claude Code task-prompt line can change thinking volume as reliably as effort.** The guide says prompt lines are less reliable than effort; it gives no Claude Code measurement.
- **Whether the Opus 5 finding "changing effort does not reliably shorten responses" holds on Opus 5.5.** The Opus 5.5 pages are silent, so C8 relies on `[O5]`.

## 5. Differences from the Sonnet 5 rubric

Where Opus 5.5 guidance differs from a `sonnet5-cc-1.0` criterion, the rubric follows Opus 5.5.

| Sonnet 5 rubric | Opus 5.5 rubric | Why |
|---|---|---|
| C1 is premised on literalism: the model "does not silently generalize", so repeated scope must be enumerated. | C1 rewards a complete up-front spec and a done condition. Enumeration is credited as completeness, not as a defence against under-generalisation. | Opus 5 "performs best when given the complete task specification up front and left to run" [O5 § Capability improvements]. No source claims Opus literalism. |
| C10 scope discipline: the main risk is under-doing ("complete the whole ask"). | C2 scope fidelity: the main risk is expansion ("adding steps that weren't requested"). It is promoted to weight 12. | [O5 § Task scope and over-verification] |
| C7 verification rewards naming a check and does not penalise extra verification steps. | C6 rewards a done-check stated as an acceptance condition. Added verification passes and verifier subagents score down. | Opus 5 "verifies its own work without being told to"; such instructions "cause over-verification" [O5 § Task scope and over-verification]. |
| C2 accepts the one-line nudge "This task involves multistep reasoning…" at low effort. | C4 prefers raising effort to prompt nudges, and asks for "think carefully" lines to be removed. | [O55 § Calibrate effort] [O55 § Thinking instructions in chat system prompts] |
| C2 scores "show your reasoning" as 0 in the criterion. | Reasoning written into the response is gate G2 (`blocked`). | It can be declined as `reasoning_extraction`, with no fallback [O55 § Safeguard refusals]. |
| G1 and G2 are separate prefill and sampling gates. | One gate, G1, also covers disabled thinking, `budget_tokens` and forced `tool_choice`. | [O55 What's new § Breaking changes] |
| C4 treats negatives as brittle under literalism, with an exception for safety limits. | C8 keeps positive form but grounds it in [O5 § User-facing progress updates]. C12 rewards a named avoid-list for frontend work. | "It responds well to instructions that name specific patterns to avoid" [O55 § Frontend design defaults]. |
| C12 frontend: concrete spec, or propose N directions. | C12 also accepts a named list of default styles to avoid, and iterating on it. A bare "avoid a generic look" scores 1. | [O55 § Frontend design defaults] |
| C13 fixes the update shape for a model that "gives regular, good updates". | C13 is the same idea. The Opus 5.5 lever is a one-line intent statement plus a short recap; the visibility of updates is harness-level. | [O55 § User-facing progress updates] |
| C14 phrasing tips: "are there bugs" over "does this compile", base64 dumps. | C14 is only the documented policy line (vulnerability finding allowed, high-risk dual-use not) plus the new biology classifier. The Sonnet phrasing tips are dropped. | No Opus 5.5 source gives phrasing guidance for cyber false positives. |
| C6 context economy: weight 7, tokenizer +30%. | C9: weight 6, tokenizer `undisclosed`. | No Opus 5.5 tokenizer statement in the sources. |
| C8 front-loading and turn economy is its own criterion. | Folded into C1. | Opus guidance has no separate per-turn token cost claim. |
| No delegation criterion. | C10 delegation proportionality (conditional). | Opus 5 "delegates to subagents more readily than prior models" [O5 § Controlling subagent spawning]. |
| No early-stop guidance. | C7 credits naming the stops wanted and the early stops to avoid. | [O55 § Unattended agentic runs] |
| No pasted-content criterion. | C15 (conditional). | [O55 § Mark pasted text in user messages] |
| `stop` is true on `pass`, on a gain under 3 points, or after 4 iterations. | One stop rule: loop until no gate fails and no criterion is below 3 (excluding gaps only the user can close), or five passes; it also stops on a criterion reversal (rule 2) or C14 = 0 (rule 0). Every score below 3 gets a fix. | Design choice for optimize-prompt-loop's final phase: merges the skill's pass limit with the rubric's bands. |
| Calibration targets `task` prompts only implicitly. | Calibrated for `task` prompts. The four prompt types are kept for skill-reviewer reuse. | Scope of this skill. |

## 6. Deferred

- Weights for `claude-md`, `skill` and `subagent` prompt types are carried from the Sonnet 5 rubric's type notes and are not yet calibrated.
- Whether skill-reviewer consumes the JSON schema as written.
