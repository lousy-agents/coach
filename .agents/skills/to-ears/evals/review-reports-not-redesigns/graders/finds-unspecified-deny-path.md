---
type: regex
pattern: '(?:non-admin|not an admin|isn.t an admin|unauthori[sz]ed|without admin|other than an admin)[^\n]{0,200}(?:forced|AC-1\.11)|(?:forced|AC-1\.11)[^\n]{0,200}(?:non-admin|not an admin|unauthori[sz]ed|isn.t an admin)'
flags: i
weight: 2
---
AC-1.11 states only the admin case; the review raises what happens when a non-admin or unauthorized caller requests a forced reset.
