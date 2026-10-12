---
description: The user names the target by a suffixed model ID from a different runtime model, and the source prompt is already well specified.
expected_outcome: The suffixed ID matches the Opus 5.5 row, the rubric loads, the verdict is pass, and the prompt keeps every source constraint without adding new scope.
tags: [rubric, opus55, alias, minimality]
model: claude-sonnet-5
runs: 3
max_turns: 30
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill]
---

/optimize-prompt-loop Optimize this task prompt for the target model claude-opus-5-5[1m] in Claude Code. Optimization only: do not execute it.

"""
Objective: every exported parser in src/parsers/ (6 files) validates its input with the matching zod schema from src/schemas/ and throws ParseError (src/errors.ts) on invalid input, so the import service can show users which field failed.
Context: CLAUDE.md covers the test and lint commands.
Constraints: change only files under src/parsers/ and their tests; keep every parser's return type unchanged; if a parser has no matching schema, leave it as is and list it in your summary.
Work: read CLAUDE.md, src/schemas/ and src/errors.ts first; make targeted edits; list unrelated problems at the end instead of fixing them. Commit on the current branch and leave pushing to me.
Verification: npm test passes and grep -rn "JSON.parse(" src/parsers returns nothing unguarded. If a test is wrong, say so instead of working around it.
Output: one line per file changed, then parsers left unchanged and unrelated problems.
"""
