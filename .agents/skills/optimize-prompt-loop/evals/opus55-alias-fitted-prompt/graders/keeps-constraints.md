---
type: regex
weight: 2
flags: i
pattern: '## Optimized prompt(?=(?:(?!## Notes)[\s\S])*?src/parsers/)(?=(?:(?!## Notes)[\s\S])*?zod)(?=(?:(?!## Notes)[\s\S])*?ParseError)(?=(?:(?!## Notes)[\s\S])*?src/errors\.ts)(?=(?:(?!## Notes)[\s\S])*?return type)(?=(?:(?!## Notes)[\s\S])*?no matching schema)(?=(?:(?!## Notes)[\s\S])*?CLAUDE\.md)(?=(?:(?!## Notes)[\s\S])*?unrelated problems)(?=(?:(?!## Notes)[\s\S])*?push)(?=(?:(?!## Notes)[\s\S])*?npm test)(?=(?:(?!## Notes)[\s\S])*?JSON\.parse\()(?=(?:(?!## Notes)[\s\S])*?test is wrong)'
---

Every source constraint survives inside the optimized prompt (before the Notes heading): scope, zod schemas, ParseError in src/errors.ts, unchanged return types, the no-matching-schema rule, reading CLAUDE.md first, unrelated problems listed, no push, npm test, the JSON.parse grep, and the wrong-test rule.
