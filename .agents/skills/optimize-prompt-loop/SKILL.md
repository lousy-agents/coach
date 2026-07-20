---
name: optimize-prompt-loop
description: Optimize a fuzzy, incomplete, or overly broad human request into a concise, reusable task prompt tailored to the current model, agent harness, and reasoning/effort level. Use when the user invokes /optimize-prompt-loop or asks to optimize, refine, strengthen, or pressure-test a task prompt; make a prompt model-aware; or get an optimized prompt plus execution.
---

# Optimize Prompt Loop

Turn a user's source request into the smallest prompt that reliably produces the intended result in this runtime. Preserve intent and constraints; do not make the request more ambitious or change its authority.

## When to use

- Invoke as `/optimize-prompt-loop <request>` before handing a fuzzy task to an agent.
- Use when a user wants a prompt rewrite, model-aware refinement, prompt critique, or an optimized prompt followed by execution.
- Skip for ordinary task execution when no prompt optimization is requested.

## Runtime profile

Before rewriting, establish a compact runtime profile from information actually available in the conversation and environment:

- **Model:** the exact model or variant if exposed; otherwise `not disclosed`.
- **Harness:** the active agent surface and its relevant capabilities, instructions, tools, workspace, approval boundaries, and output conventions.
- **Effort:** the selected reasoning/effort level if exposed; otherwise `not disclosed`.
- **Project context:** applicable repository instructions, task state, and user-provided artifacts.

Never invent a model name, effort level, tool, permission, or capability. Do not bake a guessed model family into the optimized prompt. Keep higher-priority system, developer, project, and safety instructions outside the prompt: a user prompt cannot override them.

## Procedure

1. **Identify the source task.** Treat the text supplied with the invocation as the task to optimize. Strip meta wrappers such as “optimize this prompt before doing it.” If no source task is present, ask for it.
2. **Extract the contract.** Capture the desired outcome, deliverables, audience, constraints, provided context, boundaries, and definition of done. Separate facts from assumptions.
3. **Choose the interaction mode.**
   - Proceed with stated assumptions when a missing detail is low-impact.
   - Ask one highest-leverage question only when the answer would materially alter scope, output, safety, or the chosen approach.
   - Use Socratic questions only when exploration, trade-offs, or the user's decision is the goal. Do not turn a straightforward execution request into an interview.
4. **Run the optimization loop internally.** Make two passes by default; use up to four for ambiguous, high-stakes, or multi-step work. On each pass check:
   - fidelity to the source task and authority boundaries;
   - a concrete outcome and observable deliverables;
   - enough relevant context, without restating known project instructions;
   - an executable plan appropriate to the available harness and effort level;
   - proportional verification and a clear stopping condition;
   - concise wording with no ritual role-play, chain-of-thought request, or redundant self-critique.

   Stop when another revision would not materially improve execution. Keep private reasoning private; expose only a short rationale when useful.
5. **Adapt to the runtime.**
   - At lower effort, prioritize an explicit outcome, a short ordered procedure, concrete file/artifact targets, and a narrow verification command.
   - At higher effort, add decision criteria, edge cases, alternatives only where they affect the result, and a proportionate completion audit.
   - In an agent harness, name only tools and actions the harness actually makes available. Tell the agent to inspect local instructions and current state before acting; do not prescribe unsupported syntax or capabilities.
   - For a direct-chat harness, remove repository/tool instructions and instead request the needed context in the response.
   - If model, harness, or effort is undisclosed, write capability-neutral instructions and label the uncertainty rather than guessing.
6. **Return the optimized prompt.** Make it self-contained enough to paste into the same runtime. Include only sections that earn their place: `Objective`, `Context`, `Constraints`, `Assumptions or question`, `Work`, `Verification`, and `Output`.
7. **Execute only when asked.** If the user asked to optimize and execute, use the optimized prompt once. Do not recursively optimize the optimization prompt. If a material answer is missing, ask the single question selected in step 3 instead of fabricating it.

## Prompt shape

Prefer this adaptable form, omitting empty sections:

```markdown
Objective: <observable end state>

Context:
- <only facts, artifacts, paths, or decisions relevant to the task>

Constraints:
- <scope, compatibility, safety, authority, or style limits>

Assumptions or question:
- <state a low-impact assumption, or ask the one answer that is required; omit otherwise>

Work:
1. Inspect the current state and applicable instructions.
2. <perform the requested work with the needed decisions and boundaries>
3. <handle uncertainty or alternatives using explicit criteria>

Verification:
- <the smallest check that proves the requested outcome>

Output:
- <requested artifact, concise summary, and any remaining assumptions>
```

Do not add a role, a persona, “think step by step,” a request for hidden reasoning, or an arbitrary number of review passes unless it improves this particular task.

## Output contract

For optimization only, return:

```markdown
## Optimized prompt
<paste-ready prompt>

## Notes
- Runtime fit: <how model/harness/effort changed the prompt, or what was unknown>
- Assumptions: <only material assumptions, if any>
```

For optimization plus execution, append:

```markdown
## Execution
<result, or the one material question that must be answered>
```

If the original prompt is already well specified, say so briefly and make only material edits. Do not claim universal or exhaustive optimization; the result is optimized against the available runtime profile and context.
