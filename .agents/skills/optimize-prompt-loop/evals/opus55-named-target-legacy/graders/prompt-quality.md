---
type: llm
weight: 2
---
Judge only the text between the "## Optimized prompt" heading and the "## Notes" heading.

PASS only if all four hold:
1. It states an observable end state or a checkable done condition for the error-handling work (not just "improve error handling").
2. It bounds the scope: what may change, and what to do with unrelated problems or problems below the reporting bar.
3. It contains none of these, whether as an instruction or as a prohibition: a persona ("you are a … engineer"), a request to show or write out reasoning, a fixed progress cadence ("every N tool calls"), a double-check or extra verification-pass ritual, or ALL-CAPS emphasis like CRITICAL/MUST.
4. It gives at least one concrete check (a command, test run, grep, or observable result).

FAIL if any one of the four is missing. Wording and formatting do not matter.
