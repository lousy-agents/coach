---
type: regex
pattern: '^(?=[\s\S]*(?:^|\n)[ \t]*- \**AC-\d)(?![\s\S]*(?:^|\n)[ \t]*- \**AC-\d+\.\d+\**:(?=[^\n]*\bshall\b)(?![^\n]*\[(?:src|inferred): [^\]\n]+\]\.?[ \t]*(?:\n|$))[^\n]*)'
weight: 2
---
Every criterion line (an `AC-` line containing "shall"; Notes lines that merely start with an ID are not criteria) ends with a `[src: …]` or `[inferred: …]` provenance tag. The pattern first requires that criteria exist, so a response with no `AC-` lines cannot pass by default.
