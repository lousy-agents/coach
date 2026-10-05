---
type: llm
weight: 2
---
Judge only the text between the "## Optimized prompt" heading and the "## Notes" heading.

The source prompt required all of the following. PASS only if every item is still present, in any wording:
1. Scope limited to src/parsers/ (6 files) and their tests.
2. Validation with the zod schemas in src/schemas/, throwing ParseError from src/errors.ts.
3. Parser return types stay unchanged.
4. A parser with no matching schema is left as is and listed in the summary.
5. Read CLAUDE.md, src/schemas/ and src/errors.ts before editing.
6. Unrelated problems are listed, not fixed.
7. Commit on the current branch; do not push.
8. npm test passes, and the grep check for unguarded JSON.parse( in src/parsers.
9. Report a wrong test instead of working around it.

FAIL if any item is missing, or if the prompt adds a new deliverable, a new file area, a persona, a request to show reasoning, or an extra verification pass. Wording and formatting do not matter.
