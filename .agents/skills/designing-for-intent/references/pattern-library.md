# Pattern library

Reusable interaction patterns for carrying intent across a handoff without
losing it. Load this file before Procedure step 4 (confidence-to-response)
and when recommending a handoff pattern in the report.

- **Progressive disclosure of reasoning** — show the one-line "why" for a
  suggestion up front, with the full inference chain available on demand,
  rather than either hiding it entirely or forcing the user through it
  unasked.
- **Propose-and-confirm** — the system states what it is about to do and
  waits for an explicit go-ahead before doing it, especially for anything
  hard to reverse.
- **Confidence-tiered friction** — the amount of confirmation required scales
  with how consequential and how reversible the action is, not with how
  confident the system is that it guessed right.
- **Visible delegation boundary** — the UI states, in the user's language,
  what has been handed off and what has not (e.g. "Coach reads this
  repository's diffs; it does not open PRs or push commits").
- **Scoped consent** — a permission or install grant is described and scoped
  to the specific artifact it applies to (one repository, one job), not
  phrased so broadly that it reads as blanket trust.

## Trust-posture fence (this repo's override — non-negotiable)

Where a pattern above, or any pattern brought in from elsewhere, conflicts
with this product's stated trust posture, **the trust posture wins.** This
skill does not infer architecture policy from a screen; it reads the posture
that has already been decided and designs within it. Concretely, for this
repository (`docs/architecture/system-overview.md`,
`docs/product/prd.md`):

- **High confidence still only proposes.** Never let a design "just do it"
  once confidence crosses some threshold. The source method this skill is
  built from allowed a generic "at high confidence, just do it" default; this
  skill deliberately replaces that default with propose-and-confirm at every
  confidence level, because this product's posture is "fail open for
  advisory developer flow but fail closed for credentials and mutation" —
  and an unreviewed high-confidence auto-action on someone else's repository
  is exactly a mutation-shaped risk. Treat that replacement as a hard
  override of the source pattern, not a case-by-case judgment call.
- **No silent mutation.** Repository-content mutation is out of scope for
  designs reviewed under this skill unless the artifact under review already
  documents "explicit developer activation" for it; do not design a flow
  that mutates without a human-visible, human-triggered step.
- **Coverage honesty over anticipatory automation.** An absent signal, an
  unanalyzed case, or a low-confidence guess must be represented as absence
  or uncertainty, never smoothed over by inventing a plausible-looking
  automated action to fill the gap.
