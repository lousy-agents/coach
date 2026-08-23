# Customer Journey: TypeScript Project Analysis

Status: working product companion to [`HANDOFF.md`](../../HANDOFF.md).

This document starts with the customer outcome and judges solution choices by
whether they make that outcome easier to reach. It is not an implementation
specification, and it does not declare every locked delivery decision in the
handoff to be ideal customer experience.

## The job the customer hires Coach to do

An agentic software engineer has a working TypeScript application. They want
their coding agent to help improve its architecture, not merely produce more
code. They need to know:

1. whether Coach can usefully analyze this repository;
2. what Coach found in their actual code and why it matters;
3. what a safe, focused refactor would look like; and
4. whether that refactor improved the architectural evidence.

The desired emotional arc is confidence, not installation completion:

```text
"Can Coach help here?"
        -> "It understands enough of my project to be credible."
        -> "My agent and I know exactly what to change."
        -> "The change made the evidence better."
```

Coach succeeds when that loop is useful and repeatable. It is not enough for a
command to start, for a dependency to install, or for a fixture to produce a
prearranged finding.

## Target customer and honest starting point

The first audience is a pilot-stage agentic software engineer working in their
own TypeScript repository. They are comfortable using a terminal and a coding
agent, but they should not need to understand Coach's JavaScript packaging,
compiler APIs, process boundaries, or internal rule registries.

The journey starts with a working project, not a prepared Coach project. At
that point, any of these may be missing:

- Coach itself on `PATH`;
- a compatible Node and TypeScript environment;
- a committed Coach architecture policy;
- a supported `tsconfig` or framework pattern; or
- a clear way to compare an agent's uncommitted refactor with the prior scan.

The journey must expose those gaps early and plainly. Hiding one behind an
empty report, a mysterious dependency failure, or an analysis of different
source than the agent just edited destroys trust.

## Customer promises

These are the product promises. They should hold regardless of how the
implementation achieves them.

### One understandable entry point

The customer learns one Coach command and one clear installation path for
Coach. They are never asked to obtain, vendor, or name an additional
Coach-specific product in their application repository.

### A fast, legible fit check

Before asking the customer to invest in setup or policy authoring, Coach tells
them whether it can analyze this project meaningfully. It identifies the
selected repository and code state, the project roots/configuration it can see,
and any prerequisite or coverage gap in language a human and coding agent can
act on.

### No surprise changes or invented success

Coach explains every project-changing action before it happens. It never
silently installs a dependency, edits a manifest, uses the network during a
scan, or presents a setup failure as a clean architecture result.

### Architectural intent belongs to the customer

Coach does not pretend to infer the architecture a team wants. The customer and
their agent can establish or review an explicit policy, understand what it
covers, and see how that policy leads to a finding. A missing policy is a
visible onboarding step, not a condition silently assumed by the first command.

### The first result is useful even when it is not a defect

The first report should help the customer decide whether Coach understands
enough of their application to be worth using. A concrete boundary violation
is excellent, but a coverage/fit summary or a reachability fact can also be a
first win when it accurately explains what Coach saw. The product must not
manufacture a finding merely to create a dopamine hit.

### The agent can explain, change, and verify

Coach returns stable, specific evidence in text and structured form. The coding
agent can explain the evidence, propose a narrow refactor, run the project's
tests, and show the customer what changed after a rescan. Coach remains
advisory; it does not edit the customer's code or claim a defect it did not
prove.

### Limits are visible at the decision point

If a framework, import, source-to-sink path, or code state is outside covered
analysis, Coach says so where the customer would otherwise act on the result.
“No matched finding” is not presented as “the architecture is clean.”

## The ideal journey

| Moment | Customer question | Coach and agent experience | Evidence of success |
| --- | --- | --- | --- |
| 1. Discover | “Can I use Coach on this TypeScript app?” | The customer installs one Coach product and finds a TypeScript-project entry point without reading about internal analyzer components. | `coach` is available; the documentation makes the next command obvious. |
| 2. Ask for a fit check | “Will Coach understand enough of this repository to be useful?” | Coach identifies the selected project and code state, then reports readiness: usable now, needs an explicit prerequisite, needs policy configuration, or outside current support. | The customer knows the shortest honest next step before an analysis failure occurs. |
| 3. Establish architectural intent | “What behavior do I want Coach to check?” | The customer and agent draft or review a small policy describing project roots, layers, forbidden boundaries, and any required intermediary layer. Coach explains the policy's coverage and asks for review rather than guessing. | The customer can point to the boundary Coach will evaluate and understands how to change that boundary intentionally. |
| 4. Prepare with consent | “What must change before the scan can run?” | If a prerequisite is missing, Coach names it, explains why, and offers only actions it can actually perform. A human or interactive agent explicitly approves any project or environment change. | Setup either succeeds and returns to the scan, or stops clearly with no misleading report. |
| 5. See the project | “What did Coach actually analyze?” | Coach reports the requested revision/code state, relevant project roots, coverage, and recognized patterns before or alongside findings. | The customer can tell whether the report applies to the app and change they care about. |
| 6. Get a meaningful insight | “What should we improve?” | Coach presents a concrete layer violation, a high-confidence bypass, or an informative reachability fact with locations, explanation, and coverage context. If none exists, it says what was checked and offers the next useful action. | The customer or agent can restate the evidence without inventing semantics. |
| 7. Refactor together | “What is the smallest safe change?” | The agent inspects the evidence and local conventions, proposes a focused change, receives any required approval, and runs the project's normal tests. | The human remains in control and understands the intended architectural effect. |
| 8. Verify improvement | “Did the change fix the issue?” | Coach analyzes the same requested scope against the changed code state and makes the relevant before/after evidence legible: resolved, remaining, newly introduced, or not analyzable. | The customer sees a credible improvement or an honest remaining limitation. |
| 9. Repeat | “Can we use this on the next agent change?” | The team repeats the same small loop without re-learning setup or internal terminology. | Coach becomes a voluntary part of the agent-assisted engineering workflow. |

## The first two high-value scenarios

### A supported project with a meaningful finding

The customer installs Coach, asks for a fit check, confirms a small architecture
policy, and runs a scan. Coach recognizes the project's relevant configuration
and reports, for example, an import that crosses a forbidden boundary. The
agent explains the dependency, proposes a change that preserves the policy,
runs tests, and asks Coach to compare the changed code with the earlier scan.

The first win is not “TypeScript was installed.” It is “Coach gave my agent a
specific, credible reason to change this dependency, and we could verify the
result.”

### A project that is not ready yet

The customer asks for a fit check but is missing a compatible environment,
policy, or supported project shape. Coach says which one, what it checked, and
what the customer can do next. It does not emit an architecture report that
could be mistaken for a clean result.

This is still a successful first interaction when it saves the customer from
debugging hidden setup or trusting a report that did not apply. It becomes a
poor experience only if the remedy is obscure, forces a choice before its value
is clear, or cannot be completed in the customer's agent environment.

## The agent-assisted refactor loop

The agent's role is to turn Coach evidence into an informed, reviewable change:

1. Ask Coach to assess fit or run the configured scan.
2. Summarize the strongest applicable evidence in plain language, including
   coverage limits.
3. Inspect the referenced code and the customer's policy.
4. Propose the smallest change that addresses the evidence without weakening
   the intended architecture.
5. Obtain human approval where the agent's operating policy requires it.
6. Apply the change and run the repository's normal tests.
7. Ask Coach to evaluate the same scope against the changed source.
8. Explain the before/after result and any remaining uncertainty.

The seventh step is the pivotal product promise. It requires Coach to make the
selected source state unambiguous. The agent must never tell the customer that a
refactor was verified when Coach actually analyzed an earlier revision.

## Product questions the journey exposes

These questions are intentionally visible. A technical design may answer them,
but it must not make them disappear from the customer experience.

### Which code state is Coach verifying?

Today `coach codesignal` analyzes immutable Git revisions. That protects
reproducibility, but it means an agent's uncommitted change is not part of a
baseline rescan. The current product therefore requires a commit before Coach
can verify a refactor, and must say so explicitly.

The ideal journey may instead support an explicitly selected worktree snapshot
or another clear comparison mode. Until such a capability exists, documentation
and output must show the analyzed revision and warn when uncommitted changes
would make the result surprising. This is a product trade-off to decide, not an
implementation detail to hide.

### How does a TypeScript customer create the first policy?

Layer violations and bypasses require a customer-owned policy. The existing
project-config suggestion path is Go-only, so a TypeScript customer cannot yet
experience “Coach helped me create a starting policy” as a first-run flow.

For this epic, the journey must either explicitly begin after a reviewed,
committed policy exists or provide a supported way to create one. The latter is
not automatically in scope, but the former is a real onboarding cost and must
be documented as such.

### Is compatibility friction acceptable to the first audience?

The proposed exact TypeScript pin and interactive-only install path may be a
sound narrow-pilot choice. They are not universal customer requirements. The
pilot should measure whether customers can complete setup in their actual agent
environment, including after dependency updates and without a TTY.

### What does “useful coverage” look like before a defect appears?

The customer needs a legible answer about their framework, routing pattern,
data-access pattern, aliases, and unsupported boundaries. A report with no
violation should still tell them whether Coach saw the relevant parts of their
application and what the next useful check is.

## Experience hypotheses and evidence

| Hypothesis | Evidence that supports it | Evidence that would disprove or narrow it |
| --- | --- | --- |
| One Coach product reduces adoption friction. | Foreign-repository customers can install Coach and reach a fit check without vendor instructions or a second named download. | Customers still need to locate or debug a Coach-specific runtime component. |
| Explicit consent earns trust without killing momentum. | Interactive users understand the proposed change, complete it, and reach a scan in one focused session. | Users abandon at the setup choice, or agents cannot act because their environment has no TTY. |
| A small explicit policy is an acceptable route to architectural value. | Customers and agents can author/review a policy, understand its coverage, and get meaningful evidence. | Customers cannot create a policy without expert help or receive no value after doing so. |
| Coach creates a useful agent loop. | A customer can trace a finding to code, approve a change, and understand a before/after rescan. | The agent needs to guess at missing semantics, or the rescan evaluates a different source state than the edited code. |
| Coverage honesty increases repeat use. | Customers can explain what Coach checked and return after partial or no-finding reports. | Customers treat an empty report as a clean bill of health or abandon because fit is opaque. |

Useful pilot measures follow from those hypotheses:

- time from first invocation to a legible fit result;
- time from fit result to first meaningful evidence;
- abandonment by missing Coach, environment, policy, unsupported project shape,
  or non-interactive setup;
- percentage of customers who can identify the analyzed code state correctly;
- percentage of findings that lead to an agent proposal and a reviewed change;
- percentage of those changes with a credible rescan result; and
- voluntary repeat scans.

Do not optimize for a finding on every first run. That would encourage noisy
analysis and undermine the trust the product needs.

## Current evidence boundary

This is a desired journey, not a statement that the whole path ships today.

- The local deterministic CLI is supported by current product evidence.
- TypeScript project analysis from a released Coach binary in a foreign
  repository is the purpose of the handoff's planned epic, not established
  shipped behavior.
- TypeScript CLI layer bypass and reachability facts are planned wiring; they
  are not current CLI capability.
- The existing CLI analyzes Git revisions rather than a dirty worktree, and a
  TypeScript project config must be committed at the analyzed revision.

Release language must keep those distinctions. A release may describe the
journey only to the extent that its corresponding acceptance evidence exists.

## Delivery assumptions and technical guardrails

The following are the current proposed means of delivering the journey. They
belong in [`HANDOFF.md`](../../HANDOFF.md) and the eventual implementation
epic; they should be changed if they fail the customer promises above.

- Coach ships its small analyzer as part of Coach rather than asking customers
  to vendor a `js/semantics/` tree or download a second Coach product.
- Node and the exact supported TypeScript compiler are resolved from the
  customer's project or a supported environment manager. Preflight verifies
  compatibility before scanning.
- Any installation or manifest change requires an explicit interactive choice;
  non-interactive runs fail clearly rather than installing silently.
- The scan is offline and must keep the requested project source separate from
  host-provided compiler code, so a dirty worktree cannot silently alter a
  report for a committed revision.
- The first CLI outcomes under consideration are layer violations, TypeScript
  layer-bypass findings, and facts about possible route-to-sink reachability.

These are valuable only when they create the customer outcomes above. They are
not themselves the reason a customer adopts Coach.

## Boundaries

This document does not authorize implementation changes, issue creation, or a
broader TypeScript roadmap. It does not promise automatic code edits, a clean
architecture verdict, universal framework support, or a new active defect
signal for reachability.

Its purpose is to make customer friction visible before a solution becomes too
locked in to question.
