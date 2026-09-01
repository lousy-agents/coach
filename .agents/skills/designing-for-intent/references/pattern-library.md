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
- **Visible delegation boundary** — the surface states, in the user's
  language, what has been handed off and what has not (e.g. "this reads the
  file you selected; it does not send it anywhere else").
- **Scoped consent** — a permission or install grant is described and scoped
  to the specific artifact it applies to (one repository, one job, one
  account), not phrased so broadly that it reads as blanket trust.

## Trust-posture fence (non-negotiable)

Where a pattern above, or any pattern brought in from elsewhere, conflicts
with the host product's stated trust posture, **the trust posture wins.**
This skill does not infer architecture policy from a screen; it reads the
posture that has already been decided and designs within it.

Apply these defaults unless the host artifact states a stricter posture:

- **High confidence still only proposes.** Never let a design "just do it"
  once confidence crosses some threshold. Confidence changes how strongly
  something is suggested and how much friction gates it, never whether the
  human is asked at all before an irreversible or externally visible action
  happens.
- **No silent irreversible action.** Do not design a flow that mutates,
  grants, sends, or otherwise commits without a human-visible, human-triggered
  step, unless the artifact under review already documents explicit activation
  for that step.
- **Coverage honesty over anticipatory automation.** An absent signal, an
  unanalyzed case, or a low-confidence guess must be represented as absence
  or uncertainty, never smoothed over by inventing a plausible-looking
  automated action to fill the gap.

**When a host project states no trust posture at all**, default to
propose-and-confirm, no silent irreversible action, and coverage honesty over
anticipatory automation, and record "no stated trust posture found; defaulting
to propose-and-confirm" as an Open Question in the report, so a human sees
the substitution rather than assuming a posture was found and read.
