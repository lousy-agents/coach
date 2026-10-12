---
type: regex
flags: i
pattern: '^(?=[\s\S]*(?:^|\n)[ \t]*- \**AC-\d)(?![\s\S]*(?:^|\n)[ \t]*- \**AC-(?=[^\n]*\bshall\b)[^\n]*\b(?:3|three) (?:times|retries|attempts)\b)'
weight: 2
---
No criterion adopts the current code's 3 retries; code behavior is not intent. The pattern first requires that criteria exist, so a response with no `AC-` lines cannot pass by default.
