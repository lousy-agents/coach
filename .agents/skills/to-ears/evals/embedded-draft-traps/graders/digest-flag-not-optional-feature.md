---
type: regex
flags: i
pattern: '^(?=[\s\S]*(?:^|\n)[ \t]*- \**AC-\d)(?![\s\S]*(?:^|\n)[ \t]*- \**AC-\d+\.\d+\**: Where [^\n]*digest)'
weight: 1
---
The runtime flag is never written as an Optional-feature (Where) criterion. The pattern first requires that criteria exist, so a response with no `AC-` lines cannot pass by default.
