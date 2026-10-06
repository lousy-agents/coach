---
type: regex
flags: i
pattern: '^(?=[\s\S]*(?:^|\n)[ \t]*- \**AC-\d)(?![\s\S]*(?:^|\n)[ \t]*- \**AC-\d+\.\d+\**: If [^\n]{0,60}\b(?:click|select|press)\w* [^\n]{0,20}Mute)'
weight: 1
---
"If the user clicks Mute" is a wanted event, so it is not written as Unwanted behavior (If … then). The pattern first requires that criteria exist, so a response with no `AC-` lines cannot pass by default.
