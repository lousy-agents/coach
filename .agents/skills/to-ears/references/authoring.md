# Authoring and Reviewing EARS Requirements

> The `to-ears` skill loads this file for Draft and Review modes.
>
> The pattern templates, criterion line format, and binding rules live in [`ears-contract.md`](./ears-contract.md). This file adds the judgment behind them:
> - how to pick a pattern
> - how to combine conditions
> - what makes a requirement good
> - the omission sweep in detail
> - the review checklists
> - the failure modes that show up in practice

## Generic form and clause meaning

> `<optional preconditions> <optional trigger> the <system name> shall <system response>`

The clauses have temporal meaning, and their order matters:

- **Preconditions** say when a requirement applies. If they are false, the requirement is not active.
- **Trigger** is the event that activates an event-driven requirement once its preconditions hold.
- **Response** is what the named system must do when the conditions hold.

The keyword is a structural cue. The meaning comes from the whole sentence. Use one capitalization for EARS keywords throughout an artifact. If the repo already has a convention, follow it. Otherwise use sentence case (`When …, the …`).

## Choosing a pattern

Identify the response first. Then ask: is it always required? Triggered by an event? Required during a state? Conditional on an included feature? A response to an unwanted condition? Add only the preconditions that apply.

- **Ubiquitous** is for a continuous obligation. Do not use it just because the source sentence lacks a clear trigger. Check that the behavior really applies across the whole system scope.
- **Event-driven** is for a wanted event and its response. Name the event at the system boundary where you can: "a request arrives at the API", not "the handler runs".
- **Unwanted behavior** is for a condition the system must detect, tolerate, report, prevent, or mitigate. State the response explicitly.
- **State-driven** is for a persistent state, not a one-time event. Define how the state is entered and recognized when that affects applicability. Put the definition in a Terms and States block.
- **Optional feature** is for product or deployment variability: a variant that ships or doesn't, or a module that is installed or isn't. A runtime mode, toggle, or feature flag flipped while running is a **state**, so use `While`.

Whether behavior counts as wanted or unwanted is not always objective. A system may behave normally while it tolerates a component failure. The 2009 EARS paper treats some classifications as a matter of viewpoint. When two patterns fit, pick the one that best shows the intent, and record a one-line rationale in the artifact's notes. Never imply that the label alone settles the semantics.

## Combining conditions

Use a combination only when each clause adds a necessary, distinct condition. Keep the order Where → While → When/If so it is clear what applies when. If a reader cannot tell which condition activates which response, split the requirement into linked criteria, for example `AC-3.1` and `AC-3.2` with a note saying they share the state. Avoid nested conditionals.

## Writing a good requirement

### One obligation

Prefer one independently verifiable response per criterion. Split compound source text when it contains separate obligations, triggers, responses, or kinds of verification evidence. Do not split so aggressively that a shared condition or a necessary relationship becomes hard to see. When one source statement becomes several criteria, keep the source link on each one.

Compound: `…the system shall reject the push and return an error message.`
Split:
- `…the push gateway shall reject the push.`
- `…the push gateway shall return an error that names the protected branch.`

### Name the system and the response

- Use the agreed system or component name. Avoid "it", "the application", and switching subjects between lines.
- State observable behavior or a measurable property.
- Name the recipient, destination, or affected object when it changes the expected result.
- Do not prescribe internal design unless the design itself is a binding constraint. "Shall use a hash map" is design. "Shall return a result within 100 ms for 95% of requests" is behavior.

### Replace vague language

Each term on the contract's vague-term list needs a referenced definition or a measurable criterion. High-level stakeholder requirements may stay abstract while design is open. In that case, record the open precision as an OQ; do not pretend the template removed it. Specify units, boundaries (is the boundary value included?), tolerances, time windows, rates, and measurement conditions. Use terms from the project's glossary. **Do not invent thresholds.**

### Keep conditions precise

Clarify each of these:
- who or what produces the event
- what exactly makes the condition true
- whether conditions are simultaneous, sequential, or persistent
- whether equality includes the boundary
- what happens when data is absent, stale, malformed, contradictory, or unavailable
- how states and modes are defined
- whether the requirement applies in startup, shutdown, degraded, recovery, and maintenance modes

Do not hide applicability in surrounding prose. Do not add conditions that narrow the source requirement without evidence.

### Avoid accidental scope changes

Check pronouns, "and/or", lists, ranges, negation, modifiers, and references such as "the previous value". Confirm which noun each condition describes. A shorter sentence does not mean the same thing as the longer one. Optimize for precise, reviewable meaning.

Modal verbs carry meaning too. "Must" and "shall" bind. "Should" and "may" leave bindingness open, so converting them to `shall` is an interpretation: tag it `[inferred]` and ask. Quantifiers such as "only" can settle a complement: "Only maintainers can change settings" does establish that others cannot.

### Negative behavior

Do not infer that an undesired outcome is prohibited just because the desired outcome is required:

```text
When a connection request comes from an authorized device, the gateway shall allow the connection.
If a connection request comes from an unauthorized device, then the gateway shall deny the connection.
```

Write the second line only when a source or the user establishes the denial policy. Otherwise open an OQ. A `shall not` criterion needs a condition and a prohibited outcome that a reviewer can turn into a pass/fail check.

## Omission sweep

Run the sweep for every story or requirement group before drafting is finished. Each prompt calls for investigation. None of them licenses writing a requirement.

| Prompt | Ask |
| --- | --- |
| Invalid, unauthorized, unexpected, malformed input | What does the source say happens? Is the complement of each allow or accept rule established? |
| Missing, delayed, stale, corrupt, inconsistent data | Is there a defined response, default, or rejection? |
| Dependency, component, communication, or service failure | Does the system retry, degrade, fail closed, or report? |
| Startup, default, safe, degraded, recovery, shutdown states | Does the requirement apply in each? Are entry and exit defined? |
| Boundaries | Are inclusive and exclusive edges, limits, and empty or maximum cases specified? |
| Repeated or concurrent events | Is the response idempotent? Are ordering and duplicates specified? |
| Logging, alarm, notification, fallback, recovery | Who must learn of the event, through what channel, and within what time? |

Record a disposition for each prompt that applies:
- **covered**: name the `AC-…` IDs.
- **confirmed by the user**: write a criterion tagged `[src: user]`.
- **out of scope**: add it, with a reason, to the artifact's Out of Scope list. In a standalone run, add it to the exclusions in output section 3.
- **unresolved**: open an OQ.

Put a one-line summary in the story notes, for example: `Omission sweep: invalid token → AC-1.3; mail outage → OQ-2; concurrency → out of scope (single-use tokens)`.

## Review checklists

### Form

- [ ] The pattern matches the behavior and the condition.
- [ ] The system or component is named consistently.
- [ ] Preconditions, triggers, states, and feature conditions are explicit and correctly scoped.
- [ ] Combined conditions follow Where → While → When/If, and their relationship is clear.
- [ ] The response follows `shall` and states an obligation, not a wish.

### Meaning

- [ ] The criterion has one clear interpretation in context.
- [ ] Terms, units, thresholds, timing, and boundaries are defined or tracked as OQs.
- [ ] The response is observable or verifiable by an agreed method.
- [ ] No internal design is prescribed without an approved constraint.
- [ ] The source meaning is preserved, and the provenance tag is accurate.
- [ ] Unwanted behavior, exceptions, failure modes, and variants were swept.
- [ ] No unsupported behavior was added to make the sentence look complete.
- [ ] Compound criteria were split where that helps understanding and verification.
- [ ] Conflicts, dependencies, and open decisions are visible.
- [ ] Every OQ meets the contract's minimum severity. A `[TBD …]` that leaves a criterion untestable is at least High.

### Intent: did the rewrite change the requirement?

Compare the draft to its source. Flag any criterion that:
- adds a trigger, precondition, exclusion, feature, or behavior the source does not support
- drops an exception or a safety constraint
- turns a desired outcome into an implementation choice
- treats an internal signal that only approximates the outcome as the outcome itself
- hides an open meaning behind a well-formed sentence

## Common failure modes

| Failure mode | Why it fails | Better response |
| --- | --- | --- |
| Forcing every sentence into a template immediately | Hides ambiguity in the source and can change its meaning | Analyze, decompose, then resolve or record ambiguities first |
| Treating EARS keywords as a substitute for domain analysis | Syntax supplies no omitted conditions, definitions, or hazards | Validate meaning against sources and stakeholders |
| Choosing a pattern from a word in the source | Natural-language "if" or "when" may not match EARS semantics | Classify the actual behavior, trigger, state, and applicability |
| Treating unmentioned behavior as forbidden or as allowed | The requirement may simply be incomplete | Add only authorized behavior and record gaps as OQs |
| Calling an internal flag the user-visible response | The mapped signal may not deliver the stated outcome | Verify the proxy is valid, or test the actual outcome |
| Testing only the allow path of an access rule | An allow rule says nothing about denial | Specify and test denial separately when it is required |
| Equating 100% coverage with adequate testing | Coverage shows code ran, not that the oracle was right | Report structural coverage and requirement adequacy separately |
| Using EARS to conceal design decisions | The response becomes an implementation prescription | State behavior, and trace approved design constraints separately |
| Assuming shorter is better | Concision can remove conditions or exceptions | Optimize for precise meaning, not word count |
| Treating the pattern choice as objectively unique | The wanted/unwanted split can depend on viewpoint | Record the rationale and keep the behavior explicit |
| "Fixing" an untestable criterion by inventing a value | Turns a guess into an obligation that agents will build | Mark it `[TBD …]`, open an OQ, and ask |
