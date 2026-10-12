---
type: regex
pattern: 'AC-1\.6\b[^\n]*\b(?:trigger\w*|Ubiquitous|event|always|at all times)\b'
flags: i
weight: 1
---
AC-1.6 is reported as missing its triggering event (written as always-true although sending an email is event-driven).
