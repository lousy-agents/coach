---
type: regex
pattern: '## Optimized prompt[\s\S]*?(think step by step|show (your )?reasoning|CRITICAL|double-check|after every 3|senior TypeScript engineer)[\s\S]*?## Notes'
flags: i
match: not_contains
weight: 2
---
