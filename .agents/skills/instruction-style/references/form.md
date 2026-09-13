# Form: match the formalism to the content

**Reference content** — contracts, invariants, error conditions, API behavior,
and ordered procedures where exactly one sequence is safe. Give this structure,
and use EARS where a trigger or fault condition needs naming:

- Ubiquitous: The \<actor\> \<MANDATORY\> \<action\>.
- Event-driven: When \<trigger\>, the \<actor\> \<MANDATORY\> \<action\>.
- State-driven: While \<state\>, the \<actor\> \<MANDATORY\> \<action\>.
- Unwanted: If \<fault or condition\>, then the \<actor\> \<MANDATORY\> \<action\>.
- Optional: Where \<feature exists\>, the \<actor\> \<MANDATORY\> \<action\>.

`<MANDATORY>` is the file's declared mandatory word — `MUST`, `shall`, or a
bare imperative — and EARS substitutes it; it does not choose it. With a
bare-imperative incumbent, drop the actor slot and open the clause with the
verb: `When <trigger>, <action>`.

Two constraints on the actor slot. In a second-person file — a subagent or
agent file that opens `You are ...` — the actor is `you` and the incumbent form
is the imperative, so an EARS clause there reads `When <trigger>, <imperative>`.
Renaming the addressee to a third-person noun (`The reviewer shall ...`) inside
a file that addresses it as `you` breaks the register, and once the opening
`You review ...` sentence is rewritten too, the file no longer says who the
reviewer is. And the actor is the reader, or a component the reader controls:
a `shall` on a harness, a CI job, a tool, or a package (`Claude Code shall load
this file`, `merge shall fail`) is a description wearing a requirement's
clothes, and each one teaches the reader that the mandatory word sometimes
means "does" — the misprioritization the closed set exists to prevent. Write
those as plain facts.

Name the exception when one exists: `If the store errors (not a clean miss),
then ...`. The value of these templates is that they force you to name the
trigger, the actor, and the exception. Once those are named, ordinary prose
carries them equally well — the template is an authoring aid, not a
comprehension aid.

**Behavioral content** — how to exercise judgment, when to push back, what good
work looks like, what a role is for. Write this as prose that carries its
reasons. Do not force it into a requirement template: a disposition expressed
as a prohibition loses the judgment it was meant to enable. Prefer a positive
statement of the target over a list of banned outcomes, because a prohibition
against a failure the agent was not going to make can anchor it toward that
failure.

Compare. `Do not add comments` is a prohibition that overfires. `Write code
that reads like the surrounding code: match its comment density, naming, and
idiom` is the same intent as a disposition, and it generalizes.

Four conversions to refuse outright, because each destroys the judgment it
was standing in for:

| Leave as prose | Never render as |
| --- | --- |
| Be skeptical of a passing suite you did not run | The agent \<MANDATORY\> be skeptical |
| Match the surrounding code's taste | The agent \<MANDATORY\> match taste |
| Say so in a sentence and continue, when the request looks mistaken | If the request is mistaken, then the agent \<MANDATORY\> push back |
| Act as a thoughtful peer rather than a passive summarizer | The agent \<MANDATORY\> act as a peer |

A role, a taste, and a disposition toward pushing back are not triggers,
faults, or ordered procedures, so no EARS form fits them.
