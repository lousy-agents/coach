---
type: regex
weight: 2
flags: i
pattern: '## Optimized prompt(?=(?:(?!## Notes)[\s\S])*?src/parsers/)(?=(?:(?!## Notes)[\s\S])*?zod)(?=(?:(?!## Notes)[\s\S])*?ParseError)(?=(?:(?!## Notes)[\s\S])*?src/errors\.ts)(?=(?:(?!## Notes)[\s\S])*?(return type|signature))(?=(?:(?!## Notes)[\s\S])*?((no|without(?: an?)?|lacks? an?) matching[^\n]{0,10}schemas?|no schema[^\n]{0,30}match))(?=(?:(?!## Notes)[\s\S])*?CLAUDE\.md)(?=(?:(?!## Notes)[\s\S])*?unrelated (problems|issues|findings))(?=(?:(?!## Notes)[\s\S])*?(leave[^\n.]{0,20}push(ing)? to me|(don[''’]t|do not|never)[^\n.]{0,10}push|ask[^\n.]{0,20}before push|push[^\n.]{0,30}(confirm|approv|ask)))(?=(?:(?!## Notes)[\s\S])*?npm test)(?=(?:(?!## Notes)[\s\S])*?JSON\.parse\()(?=(?:(?!## Notes)[\s\S])*?tests?[^\n]{0,20}(wrong|incorrect))'
---

Every source constraint survives inside the optimized prompt (before the Notes heading): scope, zod schemas, ParseError in src/errors.ts, unchanged return types, the no-matching-schema rule, reading CLAUDE.md first, unrelated problems listed, no push, npm test, the JSON.parse grep, and the wrong-test rule.
