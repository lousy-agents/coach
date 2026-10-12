---
description: The user names Claude Opus 5.5 as the target for an inherited prompt full of legacy scaffolding.
expected_outcome: Step 6 loads the Opus 5.5 rubric, the scaffolding is removed, the output contract is kept, and Notes ends with the Rubric line.
tags: [rubric, opus55]
model: claude-opus-5-5
runs: 3
max_turns: 30
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill]
---

/optimize-prompt-loop Optimize this request for Claude Opus 5.5 running in Claude Code. Optimization only: do not execute it.

"""
You are a senior TypeScript engineer. CRITICAL: you MUST use the Grep tool before editing. Improve the error handling in the API layer. Think step by step and show your reasoning. After every 3 tool calls, summarize progress. Don't be verbose. Double-check everything before you finish. Only report serious problems you find.
"""
