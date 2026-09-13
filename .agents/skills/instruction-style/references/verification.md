# Measuring without manufacturing defects

## A negative result is a hypothesis, not a finding

Your instrument is likelier to be wrong than the artifact. Before reporting
anything as removed, dead, missing, or miscounted, confirm the absence a second
time by a different means: search for the concept rather than the exact string,
resolve the path from its real base, read the surrounding text. Report a defect
only when two methods agree.

This is a general rule because the specific ways a check lies cannot be
enumerated. Three that have already produced false defects against correct
files:

- An identifier is **re-backticked** — a bare `lint` becoming `npm run lint` —
  and an exact-group search reads it as dropped.
- A relative link resolves **from its containing file's directory**, so every
  link in a relocated page reads as dead when checked from the repository root.
- Markdown emphasis and other spans **cross line boundaries**, so a line-based
  tool such as `grep` undercounts them. Read the whole file when the structure
  you are counting is not line-bounded.

Assume there is a fourth.

## Reporting a number

Every count you report is accompanied by the command that produced it. A number
without its command is not evidence: the next reader can only take it on faith,
or recompute it by another method and find a difference neither of you can
resolve.

A before-and-after comparison runs the **identical** command on both revisions,
not an equivalent one. Two detectors differing by one pattern produce two rates
that are not comparable, and the difference reads as a change in the artifact.

**Validate a detector against the text before reporting the rate it produces.**
Read a sample of what it flagged. A rate drives decisions, so a detector that
over-reports by missing one phrasing — a reason carried in the next sentence
rather than by a "because" — turns a small gap into a manufactured problem and
buys a fix nothing needed.

## The before-and-after table

Run one command on both revisions and report the table it prints. The columns
are the keep list plus the size and structure signals a revision pass moves:

```sh
# usage: table <before-rev> <after-rev> <file>...   (run inside the repository)
table() { B=$1; A=$2; shift 2
  c() { printf '%s' "$1" | grep -oiE "$2" | wc -l | tr -d ' '; }
  printf '%-40s %7s %7s %7s %7s %7s %7s %7s %7s %7s %7s %7s %7s\n' file wordsB wordsA bulB bulA rsnB rsnA intB intA advB advA mustB mustA
  for f in "$@"; do b=$(git show "$B:$f"); a=$(git show "$A:$f")
    printf '%-40s %7s %7s %7s %7s %7s %7s %7s %7s %7s %7s %7s %7s\n' "$f" \
      $(printf '%s' "$b" | wc -w | tr -d ' ') $(printf '%s' "$a" | wc -w | tr -d ' ') \
      $(c "$b" '^\s*- ') $(c "$a" '^\s*- ') \
      $(c "$b" '\b(so that|because|otherwise|which is why|rather than)\b') $(c "$a" '\b(so that|because|otherwise|which is why|rather than)\b') \
      $(c "$b" '\b(deliberately|intentionally|by design)\b') $(c "$a" '\b(deliberately|intentionally|by design)\b') \
      $(c "$b" '\b(silently|verbatim|explicitly|exactly)\b') $(c "$a" '\b(silently|verbatim|explicitly|exactly)\b') \
      $(c "$b" '\b(must|shall)\b') $(c "$a" '\b(must|shall)\b')
  done; }
```

Add a column per declared modal when the set is not `must`/`shall`. Read the
table as signals, not verdicts: a reason reworded into the next sentence is
preserved even though `rsn` drops by one, so a drop is a place to read, and a
drop to zero in `int` or `adv` is a keep-list loss until reading proves
otherwise. On one measured pass the core's intent markers went 4 to 0, every
subagent file lost its second person, one list went from 4 bullets to 33, and
none of it appeared in the report because no table was required.

## Sweeping for out-of-set modals

The sweep is parameterized by the file's declared triple. Build the pattern
from the candidate obligation words **minus** the file's declared mandatory
word, and match case-sensitively wherever the house style is a capitalized key
word: `grep -i` for `must` in a `MUST`-house file flags every legal `MUST`, and
an agent that "closes the set" on those hits deletes valid obligations. A
`MUST`-house fixture containing only `MUST`, `SHOULD`, and `MAY` produces zero
hits from its recipe; if it does not, the recipe is wrong, not the file.

```sh
f=path/to/file.md

# MUST-house (declared: MUST / SHOULD / plain fact). Lowercase `must` stays a
# candidate because RFC 2119 gives force only to the capitalized form.
grep -nE '\b([Ss]hall|must|has to|have to|ought to|needs? to)\b' "$f"

# shall-house (declared: shall / should / plain fact)
grep -nE '\b([Mm]ust|MUST|has to|have to|ought to|needs? to)\b' "$f"
```

For a bare-imperative core, sweep the union of both lists. Then read every hit;
the count over-reports by design:

- A **descriptive** use states how the world behaves — "a list that has to be
  updated in repository settings" — and is not an obligation on the reader.
- A **quoted** use, such as an RFC 2119 key-words paragraph or a cited
  standard, is not this file's modal and is never converted.
- `need to` is the highest-yield false positive: it sits in disposition prose
  the keep list protects ("agents need to see the reason"). Convert only a
  deontic use, and only into the declared set.

## Presenting two or more options

This shape is for a **report to a human**. Never write it into an instruction
file, and never carry its voice there: a decision record and a durable
instruction are different genres, and the close-formula below reads as
boilerplate once it lands in a file an agent loads every session.

- Analyze each option on its own.
- State its mechanism, then its limits, operational cost, and failure domain.
- Close each option with the condition that decides it: `While <context>, this
  option is the better choice. If <constraint> is violated, it fails.`
- Close the set with one table: Complexity, Failure Surface, Resource Demand.
- Give a recommendation. Do not present a survey.
