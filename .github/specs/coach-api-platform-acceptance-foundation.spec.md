# Feature Zero: Offline Acceptance Foundation

## Problem Statement

The platform groundwork specs require evidence at several public boundaries: GitHub OAuth identity, GitHub App repository reads and authorization, queue redelivery, the agent loop, and the Compose workflow. The repository already has valuable fast Ginkgo/Gomega acceptance suites that exercise public Go and command boundaries offline, but it has no shared platform-level harness for the external dependencies those later capabilities introduce. If each epic invents its own `httptest` server, container wiring, credentials, clocks, and task names, the resulting tests will be inconsistent, accidentally reach real services, and be unable to prove the Compose workflow.

Feature Zero establishes the minimum reusable acceptance foundation before platform behavior is implemented. It is deliberately a thin vertical proof, not a platform implementation: a Coach-owned fake GitHub service, deterministic local test controls, and an offline Compose runner must prove the existing GitHub ingestion and CodeSignal paths work together without credentials or egress. Baseline Scan and PR History Scan then consume and extend that foundation.

## Personas

| Persona | Impact | Notes |
| --- | --- | --- |
| Platform Engineer | Positive | Has one composable, public-boundary acceptance harness rather than recreating fakes and Compose wiring in every platform task. |
| Test Architect | Positive | Can require real broker behavior where it matters while retaining fast in-process feedback for component contracts. |
| Platform Operator | Positive | Can run a credential-free, offline local validation that fails clearly when its required local images are missing. |
| Pilot Engineer | Indirect positive | Receives platform behavior backed by tests that do not silently depend on a maintainer's GitHub, AWS, or model credentials. |

## Value Assessment

- **Primary value**: Future — establishes the executable acceptance boundary that makes the platform's auth, ingestion, queue, agent-loop, and report contracts independently verifiable and reproducible.
- **Secondary value**: Operational trust — ensures local and CI validation does not borrow ambient developer credentials or public-network availability.
- **Guardrail**: Feature Zero must prove an existing user-visible analysis path, so it cannot become infrastructure-only scaffolding.

## Binding Offline Contract

All platform acceptance tests shall run completely offline after their required container images and third-party dependencies have been acquired. An acceptance test shall not require real GitHub, AWS, OAuth, GitHub App, or model-provider credentials; ambient developer credentials from environment variables, credential files, or instance metadata; or outbound network access.

The test system shall use the most production-representative substitute that remains fully local:

| Dependency type | Required testing approach |
| --- | --- |
| GitHub and other services without a high-quality local implementation | Coach-owned test double/fake |
| Redis, Postgres, SQS, and other services that can run locally | Real Docker containers |
| Model gateway in routine CI | Deterministic local stub |
| Real llama.cpp behavior | Explicit operator-run local validation, not a required CI dependency |

Generated or test-only keys, JWTs, OAuth codes/tokens, and GitHub App installation credentials shall be used in every test fixture. No fixture, image, compose file, task, or test helper may embed or read a real credential.

An ambient-credential guard shall run before an acceptance process starts. It shall reject or scrub recognized GitHub/AWS credential environment variables, credential-file locations, and instance-metadata endpoints; record that guard decision; and fail the test when a prohibited credential source would be usable. The guard shall make an accidental public GitHub request observable and failing, not merely discouraged.

The Compose topology shall disable outbound network access for the test runner and application services. It shall use only an internal Compose network for fake GitHub, model stub, Redis, Postgres, and LocalStack where applicable. Compose tasks shall not implicitly pull images: they shall run in an offline/no-pull mode and fail with an actionable message naming a missing local image and the documented pre-acquisition step.

## User Stories

### Story 0.1: Run acceptance checks with no real credentials or egress

As a **Platform Engineer**,
I want **the platform acceptance commands to use only generated credentials and local dependencies**,
so that I can **trust a green run on an airplane, in CI, or on a clean workstation**.

#### Acceptance Criteria

- When an acceptance task starts, the harness shall activate the ambient-credential guard before it creates an HTTP client or starts Compose services.
- When GitHub, AWS, OAuth, GitHub App, or model-provider credentials are present in the environment, credential files, or instance metadata configuration, the harness shall scrub or reject them and record the result; it shall never use them as a fallback.
- When an application or test process attempts an outbound request outside the internal Compose network or an explicitly injected in-process fake transport, the acceptance run shall fail and record the attempted destination.
- When a required local image is absent, the Compose task shall fail before startup with an actionable message and shall not pull the image implicitly.
- When the same fixtures and generated keys are used, repeated acceptance runs shall produce deterministic protocol observations and versioned/golden report fixtures, except for explicitly normalized generated identifiers.

### Story 0.2: Exercise GitHub contracts through a Coach-owned fake

As a **Platform Engineer**,
I want **one fixture-driven fake GitHub service that models only Coach's required GitHub contracts**,
so that I can **test identity, repository authorization, and ingestion without public GitHub**.

#### Acceptance Criteria

- The system shall provide a test-only, Coach-owned fake GitHub service. It is not a generic GitHub emulator and shall implement only contracts required by Coach.
- Given a fixture, the fake shall support GitHub OAuth authorization-code authorization, token exchange, and `GET /user`.
- Given a fixture, the fake shall support GitHub App installation-token exchange, repository-to-installation resolution, effective collaborator permissions, repository tree/content reads, PR listing, and changed-file reads.
- Given a fixture-defined scenario, the fake shall return deterministic not-found, authorization-failure, transient-failure, and oversized-response outcomes for every supported contract where the outcome is meaningful.
- The fake shall record every request, its selected fixture/scenario, and its credential/authentication mode. Feature Zero shall provide recording sufficient to prove OAuth-token separation at the GitHub boundary: OAuth access tokens appear only on identity login paths; repository reads and repository authorization use GitHub App installation credentials; and misuse (OAuth token on a repository endpoint, Coach JWT sent to GitHub, installation credential used where an OAuth token belongs) is a recorded test failure. The actual `/v1` rejection of GitHub OAuth tokens as bearer credentials is owned by Baseline Scan once `coach-api` exists; Feature Zero does not invent that API solely to host the assertion.
- The fake shall expose a failing assertion/record when any request would reach the public GitHub API. A platform acceptance run shall assert that no such request occurred.
- For fast component acceptance tests, packages may retain in-process fakes and custom transports, including the existing `pkg/githubingest` public-boundary style. For Compose-level acceptance tests, the fake GitHub service shall run as a container on the internal Compose network, not as a host-only `httptest` dependency.

### Story 0.3: Compose the production-representative local workflow

As a **Platform Operator**,
I want **an offline Compose acceptance topology with real local infrastructure where it is meaningful**,
so that I can **validate workflow behavior without substituting hand-written fakes for broker semantics**.

#### Acceptance Criteria

- Existing package and public-API acceptance suites shall remain fast and in-process, preserving their Ginkgo/Gomega public-boundary philosophy.
- HTTP contract acceptance shall run at the public HTTP boundary with controlled fakes and deterministic state.
- Platform workflow acceptance shall run against real local Compose services: API, worker when those binaries exist, Postgres, Redis Streams, deterministic model stub, fake GitHub, fixture repository, and an external test runner. Until Baseline Scan creates `coach-api` and `coach-worker`, Feature Zero shall not invent placeholder binaries merely to claim end-to-end coverage.
- Queue-provider conformance shall run the same black-box contract against real Redis Streams and LocalStack-backed SQS. It shall not replace either provider with a hand-written queue fake. The harness shall be capable of proving worker exclusivity and crash recovery at the broker/port boundary (see Story 0.4); job-row fenced writes and attempt-scoped findings cleanup remain Baseline Task 3 assertions that consume this harness.
- Operator smoke shall remain a narrow credential-free Compose path and shall not substitute for the richer platform workflow acceptance suite. It becomes a runnable Baseline consumer when `coach-api` / `coach-worker` exist; Feature Zero defines the category only.
- Native llama.cpp validation shall be a separate operator-run check. It shall require schema-valid agent output while avoiding assertions on brittle natural-language wording; it is not a routine CI dependency. It becomes runnable when Baseline's model path exists; Feature Zero defines the category only.

### Story 0.4: Make asynchronous behavior and reports deterministic

As a **Test Architect**,
I want **time, model behavior, and report fixtures controlled by the harness**,
so that I can **prove recovery and provenance behavior without arbitrary sleeps or model prose snapshots**.

#### Acceptance Criteria

- The platform shall inject clocks and durations for heartbeats, stale-job recovery, queue redelivery, and reconciliation. Acceptance tests shall advance a controlled clock or wait on named observable conditions; they shall not rely on arbitrary sleeps.
- The queue conformance contract shall exercise actual broker redelivery, acknowledgement, duplicate delivery, poison-task handling, multi-worker behavior, and graceful shutdown through real Redis Streams and LocalStack-backed SQS. Explicitly, the harness shall prove: (1) two concurrent workers never process the same job attempt at the same time; (2) a worker can be terminated after partial handler-side persistence of work associated with an attempt; (3) after reclaim/redelivery, the completed outcome for that job is a single successful completion with no duplicate handler effects observable at the harness boundary. Feature Zero owns these reusable harness capabilities; Baseline Task 3 owns the job-specific fenced-write and attempt-cleanup assertions (increment `attempt`, delete prior findings/diagnostics, report reads final attempt only).
- Agent-loop acceptance shall use a scripted deterministic model gateway and a recording tool registry. It shall prove that handler-driven analysis and rubric paths execute through `internal/agentloop`, rather than handlers bypassing the loop with direct package calls.
- The report contract shall have golden, versioned fixtures that distinguish `source=deterministic` from `source=agent` and prove additive report evolution without changing prior report-version fixtures.

## Design

> Engineering standards: `AGENTS.md` (inlined into `CLAUDE.md`). Every implementation task below must start with a failing acceptance test at the most meaningful public boundary; the existing acceptance suites remain the model for fast Ginkgo/Gomega coverage. All commands are `mise` tasks.

### Test layers and task categories

The layers are distinct and composable. A lower layer is not evidence that a higher layer has run.

Feature Zero **defines the full eventual taxonomy** below so later epics do not invent alternate task names. Only layers marked **runnable in Feature Zero** must have executable `mise` consumers at Feature Zero completion. Layers marked **defined; Baseline consumer** are named and reserved here; they become runnable when Baseline (or a later epic) creates the binaries/paths they require. Feature Zero shall not invent placeholder `coach-api` / `coach-worker` services merely to make a reserved category green.

| Layer | Boundary and dependencies | Required task category | Runnable when |
| --- | --- | --- | --- |
| Fast acceptance | Existing public Go package/CLI boundaries, in-process fakes/transports | `test-acceptance-fast` | **Feature Zero** (existing suites + new harness unit contracts) |
| Thin offline Compose proof | External runner + fake GitHub + `pkg/githubingest` + CodeSignal; no API/worker binaries | part of `test-acceptance` / dedicated thin-proof task | **Feature Zero** (Task 0.3) |
| HTTP contract acceptance | Public HTTP routes, controlled fakes, deterministic store/time | `test-acceptance-core` | **Baseline** once `coach-api` exists |
| Platform workflow acceptance | Compose API + worker + Postgres + Redis Streams + deterministic model stub + fake GitHub + fixture repository + external runner | `test-acceptance` (full workflow leg) | **Baseline** once API + worker exist |
| Queue-provider conformance | One black-box queue contract against real Redis Streams and LocalStack SQS | `test-queue-conformance` | Harness **defined in Feature Zero**; executable legs green when Baseline Task 3a adapters exist |
| Operator smoke | Narrow credential-free Compose submission/poll path | a `platform-smoke`-style task retained for operators | **Baseline** once smoke path exists |
| Native model validation | Operator-run Compose/native llama.cpp validation, schema-focused | an explicit `platform-llm-validate`-style task | **Baseline** / operator once model path exists |

Exact task names may follow the repository's final `mise` naming convention, but their layer and responsibilities shall remain separate. When a selected environment declares a provider or workflow leg available, `test-acceptance` shall not silently omit it; until those consumers exist, CI shall run the Feature Zero-runnable subset only.

### Fake GitHub boundary

The fake GitHub service shall be fixture-driven and version its fixture schema. Fixtures shall define identities, OAuth codes/tokens, GitHub App/installation credentials, repository installation mappings, effective permissions, trees/files, pull requests/files, and named failure scenarios. The implementation shall keep OAuth identity and installation repository-read paths visibly separate, matching [ADR-001](../../docs/architecture/ADR-001-coach-api-authentication.md), [ADR-002](../../docs/architecture/ADR-002-identity-separate-from-repo-reads.md), and [ADR-003](../../docs/architecture/ADR-003-repository-authorization-policy.md).

Request recording is part of the contract, not debug logging. The recorder shall allow a test to assert the exact sequence and authentication mode for OAuth authorization/token/`/user`, installation-token issuance, installation resolution, collaborator permissions, tree/content reads, PR listing, and changed-file reads. It shall make GitHub-boundary token leakage detectable: an OAuth token used for a repository endpoint, or a Coach JWT sent to GitHub, is a test failure against the fake's record. Proving that `/v1` rejects a GitHub OAuth token as `Authorization` bearer is a Baseline Scan acceptance assertion once the API exists; the fake's recording capability must be sufficient for that consumer without Feature Zero hosting the route.

### Deterministic controls and real dependencies

Fakes are allowed only at dependencies without a sufficiently representative local implementation. `TaskQueue` component tests may use an in-process fake to isolate an API or worker unit, but the portable queue contract is authoritative only when run against real containers. This implements [ADR-006](../../docs/architecture/ADR-006-watermill-queue-abstraction.md)'s conformance requirement through this harness; Redis/SQS adapter behavior itself remains work for the queue epic. The harness must expose hooks (or equivalent observable controls) for dual-worker exclusion, worker-kill mid-attempt, and post-reclaim single-completion checks at the queue/port boundary. Job-domain attempt fencing and findings deduplication remain Baseline Task 3.

The model stub shall be scripted for component/HTTP acceptance and deterministic for Compose workflow acceptance. A recording tool registry shall be injectable at the `internal/agentloop` boundary. It shall record handler-driven and model-selected calls, reject unregistered calls, and allow assertions that the deterministic path did not bypass the registry.

Clocks and duration configuration shall be dependencies at the queue, worker lifecycle, and reconciler boundaries. Tests shall use explicit clock advancement and broker-visible state transitions, not `time.Sleep` polling. Generated keys/tokens may be fresh per run but shall be emitted only in test-process memory or disposable test files and normalized out of goldens.

### Thin executable proof

Feature Zero's proof must use currently available capabilities or the earliest minimally required additions:

1. Start fake GitHub offline with a fixture and generated GitHub App test credentials.
2. Exercise `pkg/githubingest` through its public boundary against that fake for a fixture repository/content read.
3. Feed the fixture content through the existing CodeSignal analysis path and assert a deterministic, versioned report fixture.
4. Run that proof from an external Compose test runner with outbound network disabled and no real credentials, asserting the fake-GitHub request record and credential guard result.

This proof shall not require nonexistent `coach-api` or `coach-worker` binaries. Baseline Scan becomes the first consumer that extends it into an API → queue → worker workflow.

### Dependencies

- **Baseline Scan** consumes Feature Zero for fake GitHub, the offline Compose acceptance topology, deterministic clock/duration controls, report-golden conventions, and acceptance task categories.
- **PR History Scan** extends the same fake-GitHub fixture schema with PR lists and changed files. It shall not create a second GitHub fake, separate credential recorder, or alternate Compose test approach.
- [ADR-006](../../docs/architecture/ADR-006-watermill-queue-abstraction.md)'s black-box Redis/SQS provider conformance requirements are executed through Feature Zero's harness; implementing the adapters remains Baseline Scan Task 3a work.

## Tasks

### Task 0.1: Define the acceptance harness contract and offline guard

**Depends on**: none

**Objective**: Define shared fixture/recording interfaces, generated test credential helpers, ambient-credential guard, no-egress policy, controlled clock seam, and `mise` task categories without changing production platform behavior.

**Requirements**:

- Preserve the existing Ginkgo/Gomega public-boundary suites and make their place in `test-acceptance-fast` explicit.
- Document the full task-category taxonomy (fast, thin Compose proof, HTTP core, workflow, queue conformance, operator smoke, native-model) while wiring runnable `mise` consumers only for Feature Zero-runnable layers; reserve names for Baseline consumers without placeholder binaries.
- Add a guard test that is red before implementation when a known GitHub/AWS ambient credential source or external request is attempted.
- Document no-pull/offline Compose preflight behavior and the required local image acquisition step.
- Define golden fixture versioning and normalization rules for generated identifiers.

**Verification**:

- [ ] The guard rejects injected ambient credential and egress scenarios; red first, then green
- [ ] `mise run test-acceptance-fast` (or final equivalent) runs the existing fast acceptance suites
- [ ] Relevant existing `mise` Go checks pass

**Done when**:

- [ ] No acceptance task can silently inherit a developer credential or public-network route
- [ ] Controlled clock and fixture/recording conventions are reusable by later tasks

### Task 0.2: Implement the Coach-owned fake GitHub service

**Depends on**: Task 0.1

**Objective**: Implement the fixture-driven fake GitHub service and request/credential recorder for the Coach contracts only.

**Requirements**:

- Start with a failing public-boundary acceptance test for OAuth authorization-code/token/`/user`, GitHub App installation-token flow, repo-to-installation resolution, effective permissions, and repository tree/content reads.
- Support deterministic named not-found, authorization, transient, and oversized scenarios.
- Run in-process for fast tests and as an internal-network Compose service for workflow tests using the same fixture schema and recorder contract.
- Assert no public GitHub request and no OAuth token repository read via the recorder. Provide recording/fixtures sufficient for Baseline to assert `/v1` rejects GitHub OAuth tokens as bearers once `coach-api` exists; do not invent that API in Feature Zero.

**Verification**:

- [ ] Fixture scenarios and recorded authentication modes are asserted through public consumers; red first, then green
- [ ] `pkg/githubingest` remains exercised through its exported API, not package internals
- [ ] Relevant `mise` checks pass

**Done when**:

- [ ] One Coach-owned fake supports all shared GitHub contracts needed by Baseline Scan
- [ ] The fake is clearly constrained, versioned, and not positioned as a generic emulator

### Task 0.3: Establish the thin offline proof and Compose runner

**Depends on**: Tasks 0.1–0.2

**Objective**: Run the fake-GitHub → `pkg/githubingest` → existing CodeSignal proof from an external offline Compose runner.

**Requirements**:

- Start with a failing Compose acceptance test proving fixture content is read through `pkg/githubingest`, analyzed through the existing CodeSignal path, and represented by a versioned deterministic report fixture.
- Run fake GitHub as a Compose service on the internal network; mount/provide only fixture data and generated test credentials.
- Prove the runner completes with outbound network disabled, ambient credentials rejected/scrubbed, no public GitHub requests, and no required platform API/worker binary.
- Fail clearly when a required local image is missing; do not pull it.

**Verification**:

- [ ] Offline Compose proof is red first, then green
- [ ] Fake request record proves installation credentials were used for ingestion
- [ ] Report golden is deterministic and versioned

**Done when**:

- [ ] The repository has executable evidence that Feature Zero is not scaffolding theater
- [ ] Baseline Scan can attach API/worker services without redesigning the runner or fake

### Task 0.4: Add provider conformance and agent-loop harness seams

**Depends on**: Task 0.1; may proceed while later platform adapters are implemented

**Objective**: Define the reusable black-box queue contract plus scripted-model/recording-tool-registry harnesses consumed by platform tasks.

**Requirements**:

- Define (and, when adapters exist, execute) the portable queue behavior against real provider containers; do not satisfy provider conformance with queue fakes.
- Harness requirements include dual-worker exclusion (same job attempt never processed concurrently), worker termination after partial persistence, and post-reclaim single-completion without duplicate handler effects at the harness boundary. Baseline Task 3 remains the owner of fenced writes and attempt-scoped findings/diagnostics cleanup assertions.
- Make controlled clocks/durations available for heartbeats, stale reclaim, redelivery, and reconciliation assertions.
- Make scripted model responses and tool-registry recordings available for Baseline/PR History to prove `internal/agentloop` execution and deterministic/agent provenance; Feature Zero need not run full agent-loop workflow without platform handlers.
- Keep native llama.cpp validation an operator-only, schema-validity category rather than a wording golden or mandatory CI dependency; runnable only when Baseline's model path exists.

**Verification**:

- [ ] Queue conformance harness contract documents dual-worker exclusion, kill-mid-attempt, and single-completion-after-reclaim; red-first executable legs when adapters exist
- [ ] `mise run test-queue-conformance` (or final equivalent) selects real Redis Streams and LocalStack SQS when adapters exist
- [ ] Scripted-model / recording-tool seams are injectable for later consumers; full bypass proofs land with Baseline handlers
- [ ] Goldens distinguish deterministic and agent findings and preserve prior report versions

**Done when**:

- [ ] Baseline Scan Task 3a can implement ADR-006 validation without inventing a separate harness
- [ ] Baseline Task 3 can attach fenced-write / attempt-cleanup acceptance on top of the shared crash-recovery harness hooks
- [ ] Baseline/PR History handlers can prove agent-loop routing without test-only production shortcuts

## Out of Scope

- A generic GitHub emulator or support for GitHub contracts Coach does not consume.
- A platform HTTP API, worker, queue adapter, model backend, or production implementation of any Baseline Scan behavior.
- Replacing real Redis/SQS provider conformance with fake queue tests.
- Replacing existing fast public-boundary acceptance suites with Compose tests.
- Requiring real GitHub, AWS, OAuth, GitHub App, model-provider credentials, public egress, or implicit image pulls.
- Making the narrow operator smoke a substitute for the full platform workflow suite.
- Making native llama.cpp a mandatory CI dependency or asserting brittle natural-language output.

## Verification and Done When

Feature Zero is complete only when:

- [ ] The thin proof runs entirely offline after documented image/dependency acquisition and proves no real credential or public-network use.
- [ ] A fixture-driven Coach-owned fake GitHub service and recorder support the shared OAuth, installation, authorization, and repository-read contracts.
- [ ] The full task taxonomy is defined; only Feature Zero-runnable layers (fast acceptance + thin offline Compose proof, plus harness definition for queue conformance) are required green; HTTP/workflow/smoke/native-model categories are reserved for Baseline consumers without placeholder services.
- [ ] The queue harness is ready to execute ADR-006's real Redis/LocalStack contract—including dual-worker exclusion, kill-mid-attempt, and single-completion-after-reclaim—as adapters arrive; the adapter implementation and job-domain fencing assertions themselves are not claimed complete by this feature.
- [ ] Golden/versioned report conventions prove deterministic-versus-agent provenance and additive report evolution.
- [ ] The Baseline and PR History specs explicitly consume this foundation and retain acceptance-test-first requirements.
