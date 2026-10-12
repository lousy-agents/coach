---
description: Control case. The user names a target model that has no rubric, so the result does not depend on the runtime model (a `--model` override cannot turn step 6 on).
expected_outcome: Step 6 is skipped. No rubric file is read, no Rubric line is added, and the output contract is unchanged.
tags: [control]
model: claude-sonnet-5
runs: 3
max_turns: 30
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill]
---

/optimize-prompt-loop Optimize this request for Claude Sonnet 5 running in Claude Code. Optimization only: do not execute it.

"""
You are a senior TypeScript engineer. CRITICAL: you MUST use the Grep tool before editing. Improve the error handling in the API layer. Think step by step and show your reasoning. After every 3 tool calls, summarize progress. Don't be verbose. Double-check everything before you finish. Only report serious problems you find.
"""
