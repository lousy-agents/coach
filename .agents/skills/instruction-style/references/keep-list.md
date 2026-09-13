# What not to remove

A revision pass deletes the highest-value words first unless it is told not to.
Length is not the target; never justify a deletion by word count. Preserve, and
restore if a prior pass removed them:

- **Reason clauses**: `so that`, `because`, `otherwise`, `which is why`.
- **Intent markers**: `deliberately`, `intentionally`, `by design`. These mark a
  choice an agent shall not "repair", and repairing an intentional design is the
  most expensive failure mode in a config-heavy repository. They are not
  qualifiers of degree.
- **Stance words** that set a role's disposition: `adversarial`, `skeptical`.
- **Manner adverbs that constrain the output**: `silently`, `verbatim`,
  `explicitly`, `exactly`. They are the rule, not decoration — `silently check
  that the response follows these requirements` forbids narrating the check,
  and `check` alone permits it.
- **Discriminating detail in routing text** — descriptions of skills, tools, and
  subagents. These are lookup tables, and specifics are what make routing work.
  Under-description is the common failure here, not over-description.
- **Defining clauses** that say what a term means. Without them a paragraph that
  reads as guidance becomes unactionable.
- **Examples** that pin a format-sensitive output shape.
- **Prohibitions** against failures that still occur.

**The keep list outranks any size figure.** Rationale, intent markers, and
routing discriminators are never what you cut to reach a number. A file over
its advisory figure because the excess is rationale is the tradeoff working.
Cut derivable reference first, and stop there.
