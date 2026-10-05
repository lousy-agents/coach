---
description: Control case. The runtime is a model with no rubric and the user names no target model.
expected_outcome: Step 6 is skipped. No rubric file is read, no Rubric line is added, and the output contract is unchanged.
tags: [control]
model: claude-sonnet-5
runs: 3
max_turns: 30
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill]
---

/optimize-prompt-loop Optimize this request for use in Claude Code. Optimization only: do not execute it.

"""
You are a senior TypeScript engineer. CRITICAL: you MUST use the Grep tool before editing. Improve the error handling in the API layer. Think step by step and show your reasoning. After every 3 tool calls, summarize progress. Don't be verbose. Double-check everything before you finish. Only report serious problems you find.
"""
