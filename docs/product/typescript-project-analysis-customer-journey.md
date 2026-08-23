# Customer Journey: TypeScript Project Analysis

Status: founder/product hypothesis for a pre-pilot TypeScript customer journey.

This document starts with the customer outcome and judges solution choices by
whether they make that outcome easier to reach. It should guide discovery,
product copy, and the eventual epic's acceptance criteria. It is not an
implementation specification, evidence that this persona will adopt Coach, or
a release claim. Product truth remains the [PRD](prd.md); current shipped
evidence remains the [pilot evaluation](evaluations/codesignal-pilot-readiness.html).

## The job the customer hires Coach to do

An agentic software engineer has a working TypeScript application. They want
their coding agent to help improve its architecture, not merely produce more
code. They need to know:

1. whether Coach can usefully analyze this repository;
2. what Coach found in their actual code and why it matters;
3. what a focused refactor would look like, including residual uncertainty; and
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

The candidate first persona is a project owner using a coding agent in their
own TypeScript repository. This is a founder hypothesis, not a validated market
segment: interviews and pilot use must establish whether the person has this
job, prefers this workflow, and will return after the first loop. The hypothesis
can inform the journey, its research prompts, and a narrow implementation epic;
it cannot by itself choose a durable persona, pricing, distribution model, or
broader roadmap.

This candidate customer is comfortable using a terminal and a coding agent, but
should not need to understand Coach's JavaScript packaging, compiler APIs,
process boundaries, or internal rule registries.

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

The customer can start from one obvious Coach entry point and understand the
setup it needs without learning about internal analyzer components. Whether
Coach ships as one artifact or uses another delivery arrangement is a solution
hypothesis, not this experience invariant.

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

Coach returns source-anchored evidence that a human and coding agent can trace
back to the relevant rule, location, and coverage limit. Recommendation-grade
evidence is prioritized enough to support a focused next action; aggregate or
evidence-poor output remains informational rather than pretending to direct a
refactor. The agent can propose a narrow change, assess whether relevant
behavior is protected by tests or other evidence, and show the customer what a
rescan did and did not establish. Coach remains advisory; it does not edit the
customer's code or claim a defect or behavioral guarantee it did not prove.

### Limits are visible at the decision point

If a framework, import, source-to-sink path, or code state is outside covered
analysis, Coach says so where the customer would otherwise act on the result.
“No matched finding” is not presented as “the architecture is clean.”

## Moment 0: the trigger before Coach

The journey begins when something outside Coach creates urgency: an agent has
introduced a dependency a reviewer cannot reason about, a repeated refactor is
eroding a boundary, or the project owner cannot tell whether generated code
preserved intended behavior. Today the customer and agent inspect a diff, run
the existing test suite, manually trace dependencies, or ask a reviewer to
reconstruct intent. Those workarounds can prove individual facts, but they do
not give a reproducible architectural account of the selected change.

Coach is the right intervention only if it advances that situation: it must
give the customer a clearer decision than the workaround, not merely add a new
terminal command. Discovery should test this trigger and the current workaround
before treating Coach onboarding as the journey's beginning.

## The ideal journey

| Moment | Customer question | Coach and agent experience | Evidence of success |
| --- | --- | --- | --- |
| 0. Trigger | “Why is the current review or refactor loop not enough?” | The human and agent name the architectural or review uncertainty they are trying to reduce, and the current workaround that is falling short. | The customer can say what a useful Coach result would let them decide. |
| 1. Discover | “Can I use Coach on this TypeScript app?” | The customer finds the Coach entry point without reading about internal analyzer components. | `coach` is available; the documentation makes the next command obvious. |
| 2. Ask for a fit check | “Will Coach understand enough of this repository to be useful?” | Coach identifies the selected project and code state, then reports readiness: usable now, needs an explicit prerequisite, needs policy configuration, or outside current support. | The customer knows the shortest honest next step before an analysis failure occurs. |
| 3. Establish architectural intent | “What behavior do I want Coach to check?” | The customer and agent draft or review a small policy describing project roots, layers, forbidden boundaries, and any required intermediary layer. Coach explains the policy's coverage and asks for review rather than guessing. | The customer can point to the boundary Coach will evaluate and understands how to change that boundary intentionally. |
| 4. Prepare with consent | “What must change before the scan can run?” | If a prerequisite is missing, Coach names it, explains why, and offers only actions it can actually perform. A human or interactive agent explicitly approves any project or environment change. | Setup either succeeds and returns to the scan, or stops clearly with no misleading report. |
| 5. See the project | “What did Coach actually analyze?” | Coach reports the requested revision/code state, relevant project roots, coverage, and recognized patterns before or alongside findings. | The customer can tell whether the report applies to the app and change they care about. |
| 6. Get a meaningful insight | “What should we improve?” | Coach presents a concrete layer violation, a high-confidence bypass, or an informative reachability fact with locations, explanation, and coverage context. If none exists, it says what was checked and offers the next useful action. | The customer or agent can restate the evidence without inventing semantics. |
| 7. Refactor together | “What is the smallest architecture-preserving change?” | The agent inspects the evidence, local conventions, and relevant behavioral tests; it proposes a focused change and identifies when behavioral coverage should be added or remains uncertain. | The human remains in control and understands the intended architectural effect, behavior evidence, and residual risk. |
| 8. Verify improvement | “Did the change improve the intended evidence?” | Coach analyzes the same requested scope against the changed code state and makes the relevant before/after evidence legible: resolved, remaining, newly introduced, or indeterminate. | The customer sees a credible architecture improvement or an honest limitation; it never treats an incomplete comparison as verified. |
| 9. Hand off a reviewable change | “What can a reviewer trust about this change?” | The human and agent carry the architectural intent, code-state provenance, behavior evidence, Coach evidence, coverage limits, and residual uncertainty into the change's review context. | A reviewer can evaluate the change without reconstructing the agent's reasoning from scratch. |
| 10. Repeat | “Can we use this on the next agent change?” | The team repeats the same small loop without re-learning setup or internal terminology. | Coach becomes a voluntary part of the agent-assisted engineering workflow. |

## Actor ownership and durable handoffs

The journey is a service blueprint, not a single actor's funnel. These handoffs
must be explicit in the product and in research sessions.

| Actor | Owns | Needs to understand | Durable handoff |
| --- | --- | --- | --- |
| Human project owner | The goal, selected code state, policy approval, setup consent, and refactor approval. | What Coach can and cannot establish; what the agent proposes to change. | Reviewed policy, explicit approvals, and the final review context. |
| Coding agent | Exploring code, drafting a policy or refactor, identifying relevant behavior tests, and explaining limits. | The chosen scope, approved policy, source state, and Coach evidence. | Proposed change, test/behavior evidence, and a before/after explanation. |
| Coach | Declaring fit, analyzed scope, findings/facts, diagnostics, and coverage limits without mutating the project. | Nothing implicit: the configured policy and requested source state. | Reproducible report with provenance, evidence, and diagnostics. |
| Repository and review system | Persisting source, policy, commits, tests, and the reviewable change. | Which report and test evidence correspond to the reviewed revision. | Committed source/policy and a review artifact that names evidence and uncertainty. |

## High-value scenarios

### A supported project with a meaningful finding

The customer finds Coach, asks for a fit check, confirms a small architecture
policy, and runs a scan. Coach recognizes the project's relevant configuration
and reports, for example, an import that crosses a forbidden boundary. The
agent explains the dependency, proposes a change that preserves the policy,
checks the relevant behavior is protected or recommends a missing behavioral
test, and asks Coach to compare the changed code with the earlier scan.

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

### A supported, complete scan with no finding

Coach fully analyzes the selected source and policy, but no configured boundary
or supported source-to-sink pattern produces a finding. The safe decision is
only that no configured, covered rule matched this revision; it is not that the
architecture is compliant or behavior is safe. The next useful action is to
record that baseline, review whether the policy covers the boundary the customer
cares about, or ask the agent to inspect a higher-risk flow with Coach's stated
coverage in mind.

### A supported scan with incomplete or uncertain coverage

Coach recognizes the project but cannot fully resolve a configured import,
route, source-to-sink path, or source scope. The safe decision is that Coach
cannot establish a complete architecture result for this revision. The next
action is to inspect the diagnostic with the agent, correct the supported
configuration or project shape where possible, and carry the limitation into
the review rather than treating an absent finding as a pass.

## The agent-assisted refactor loop

The agent's role is to turn Coach evidence into an informed, reviewable change:

1. Ask Coach to assess fit or run the configured scan.
2. Summarize the strongest applicable evidence in plain language, including
   coverage limits.
3. Inspect the referenced code, the customer's policy, and the tests or other
   evidence that represent affected behavior.
4. Propose the smallest change that addresses the evidence without weakening
   the intended architecture. When relevant behavior is not credibly protected,
   add or recommend a behavioral test before treating a passing test run as
   preservation evidence.
5. Obtain human approval where the agent's operating policy requires it.
6. Apply the change and run the repository's normal tests.
7. Ask Coach to evaluate the same scope against the changed source.
8. Explain separately what improved in the architecture evidence, what supports
   behavioral preservation, and any remaining uncertainty.

The seventh step is the pivotal product promise. It requires Coach to make the
selected source state unambiguous and to mark lifecycle conclusions
indeterminate when a relevant path was not completely analyzed. The agent must
never tell the customer that a refactor was verified when Coach actually
analyzed an earlier revision, skipped a changed path, or lacks the behavior
evidence needed for that claim.

## Hand off a reviewable change

Coach does not need to become a PR bot for this journey to reach its downstream
customer outcome. After a verified refactor, the human and agent should carry a
small, reviewable explanation alongside the change:

- the intent and approved architecture policy that motivated the change;
- the selected source state and comparison scope;
- the relevant behavior tests or the stated absence of behavioral evidence;
- Coach's source-anchored finding or fact and its before/after state; and
- coverage limits and residual uncertainty.

This handoff is a desired experience boundary, not a promise that the current
CLI publishes a review artifact or that a reviewer receives an automated Coach
verdict.

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

### Can Coach make a trustworthy change-comparison claim?

The agent needs more than a baseline report to claim that a refactor improved a
change. When an added, renamed, copied, or otherwise unanalyzable path prevents
a complete comparison, both human-readable and structured output must identify
the affected path and make lifecycle/counter conclusions indeterminate. A
summary that says no change was introduced while relevant source was skipped is
not an honest verification result.

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
data-access pattern, aliases, source scope, and unsupported boundaries. A
report must distinguish code included by project configuration from code that
meaningfully represents the customer's production concern, especially when a
test file can be included in the same scan. A report with no violation should
still tell them whether Coach saw the relevant parts of their application and
what the next useful check is.

## Experience hypotheses and evidence

| Hypothesis | Research status / current evidence | Evidence we would seek | Evidence that would disprove or narrow it |
| --- | --- | --- | --- |
| One obvious Coach entry point reduces adoption friction. | Unvalidated for this persona. The foreign-repository TypeScript project path is planned rather than shipped; [current product evidence](evaluations/codesignal-pilot-readiness.html) separates that path from file-local TypeScript analysis. | Customers can reach a fit check without component-specific setup archaeology. | Customers still need to locate or debug a Coach-specific runtime component. |
| Explicit consent earns trust without killing momentum. | Unvalidated. Interactive setup is a proposed delivery path, not observed customer preference. | Interactive users understand the proposed change, complete it, and reach a scan in one focused session. | Users abandon at the setup choice, or agents cannot act because their environment has no TTY. |
| A small explicit policy is an acceptable route to architectural value. | Partially supported by the [configured-layer design](../../README.md#configured-layer-violations---project-config), but unvalidated with TypeScript customers. | Customers and agents can author/review a policy, understand its coverage, and get meaningful evidence. | Customers cannot create a policy without expert help or receive no value after doing so. |
| Coach creates a useful agent loop. | Unvalidated. The [PRD success signals](prd.md#12-success-signals) treat voluntary repeat use and useful customer judgment as pilot evidence, not as established behavior. | A customer can trace a finding to code, approve a change, and understand a complete before/after rescan. | The agent needs to guess at missing semantics, the comparison is incomplete, or the rescan evaluates a different source state than the edited code. |
| Coverage honesty increases trust and repeat use. | The [CLI contract](../../README.md#configured-layer-violations---project-config) has coverage/diagnostic rules, but there is no customer evidence yet that people understand or value them. | Customers can explain what Coach checked, name the limit, and return after partial or no-finding reports. | Customers treat an empty report as a clean bill of health or abandon because fit is opaque. |

Useful pilot measures follow from those hypotheses:

- time from first invocation to a legible fit result;
- time from fit result to first meaningful evidence;
- abandonment by missing Coach, environment, policy, unsupported project shape,
  or non-interactive setup;
- percentage of customers who can identify the analyzed code state correctly;
- percentage of findings that lead to an agent proposal and a reviewed change;
- percentage of those changes with a credible rescan result; and
- voluntary repeat scans.

Pair those funnel measures with qualitative evidence from think-aloud sessions,
agent transcripts, and post-task interviews:

- Can the customer accurately explain the selected source state, what Coach
  checked, and the most important coverage limit?
- Does the customer feel in control of policy, installation, and refactor
  approval, rather than delegated around by the agent?
- Can they explain to a reviewer why the change was made, what behavior was
  checked, and what remains uncertain?
- After a credible verification, do they report relief, increased confidence,
  or another concrete moment of value—and can they name the evidence that
  produced it?

Do not optimize for a finding on every first run. That would encourage noisy
analysis and undermine the trust the product needs.

## Current evidence boundary

This is a desired journey, not a statement that the whole path ships today.

- The current self-serve evidence is narrow: a local Go baseline with configured
  layer policy, not this complete TypeScript customer journey.
- TypeScript project analysis from a released Coach binary in a foreign
  repository is the purpose of a planned epic, not established
  shipped behavior.
- TypeScript CLI layer bypass and reachability facts are planned wiring; they
  are not current CLI capability.
- The existing CLI analyzes Git revisions rather than a dirty worktree, and a
  TypeScript project config must be committed at the analyzed revision.
- Text and structured reports must both carry enough source, rule, coverage,
  comparison, and version provenance for a human and agent to make the same
  claim. Where that does not yet hold, the journey is aspirational rather than
  demonstrated.

Release language must keep those distinctions. A release may describe the
journey only to the extent that its corresponding acceptance evidence exists.

## Delivery assumptions and technical guardrails

The following are current implementation hypotheses to record in a versioned
epic. They should be changed if they fail the customer promises above.

- Coach may ship its small analyzer as part of Coach rather than asking
  customers to vendor a `js/semantics/` tree or download another component.
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
