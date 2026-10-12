---
type: llm
weight: 2
---
Judge only the text between the "## Optimized prompt" heading and the "## Notes" heading, against this source prompt:

```text
Objective: every exported parser in src/parsers/ (6 files) validates its input with the matching zod schema from src/schemas/ and throws ParseError (src/errors.ts) on invalid input, so the import service can show users which field failed.
Context: CLAUDE.md covers the test and lint commands.
Constraints: change only files under src/parsers/ and their tests; keep every parser's return type unchanged; if a parser has no matching schema, leave it as is and list it in your summary.
Work: read CLAUDE.md, src/schemas/ and src/errors.ts first; make targeted edits; list unrelated problems at the end instead of fixing them. Commit on the current branch and leave pushing to me.
Verification: npm test passes and grep -rn "JSON.parse(" src/parsers returns nothing unguarded. If a test is wrong, say so instead of working around it.
Output: one line per file changed, then parsers left unchanged and unrelated problems.
```

PASS if every file, deliverable, and behavior the optimized prompt asks for is already in the source prompt. Restating, reordering, or tightening the source's own requirements is fine. Lines that only bound or end the work — flagging a request that looks mistaken and continuing as asked, ending the turn when done or blocked on the user, an inspect-first step — are not new scope.

FAIL if the optimized prompt asks to change a file outside `src/parsers/` and their tests, or names a file, deliverable, or behavior the source prompt does not mention (for example refactoring, new tests beyond the parsers', or pushing to a remote).
