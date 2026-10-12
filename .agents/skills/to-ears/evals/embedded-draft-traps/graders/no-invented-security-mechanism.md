---
type: regex
flags: i
pattern: '^(?=[\s\S]*(?:^|\n)[ \t]*- \**AC-\d)(?![\s\S]*(?:^|\n)[ \t]*- \**AC-[^\n]*\b(?:TLS|SSL|AES|HTTPS|encrypt\w*|HMAC|OAuth)\b)'
weight: 1
---
"Notifications must be secure" is not turned into an invented mechanism inside a criterion. The pattern first requires that criteria exist, so a response with no `AC-` lines cannot pass by default.
