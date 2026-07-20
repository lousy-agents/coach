# Acceptance Harness: Test-Layer Taxonomy, Offline Compose Preflight, and Golden Fixture Rules

| Field | Value |
| --- | --- |
| Status | Accepted for Feature Zero (epic #73, issue #76 "Task 0.1") |
| Date | 2026-07-20 |
| Source spec | [`.github/specs/coach-api-platform-acceptance-foundation.spec.md`](../../.github/specs/coach-api-platform-acceptance-foundation.spec.md) ("Feature Zero: Offline Acceptance Foundation") |

This is a reference doc, not a restatement of the spec: it records the decisions Task 0.1 closes out (the full task-category taxonomy, the no-pull/offline Compose preflight contract, and golden-fixture conventions) and points at the spec for the full rationale. See the spec for personas, value assessment, and the complete task list (0.1–0.4).

## 1. Test-layer taxonomy

Feature Zero defines the **full eventual taxonomy** so later epics (Baseline Scan, PR History Scan, and beyond) don't invent alternate task names or re-litigate layer boundaries. Only the first row has a real, runnable `mise` task today. The rest are named and reserved here; Feature Zero shall not invent placeholder `coach-api`/`coach-worker` services merely to make a reserved category green (spec, "Test layers and task categories").

| Layer | Boundary | Task category | Status in this repo today |
| --- | --- | --- | --- |
| Fast acceptance | Existing public Go package/CLI boundaries, in-process fakes/transports | `mise run test-acceptance-fast` | **Runnable now** (the six existing `*Acceptance` suites plus `internal/acceptanceharness`'s own contract tests) |
| Thin offline Compose proof | External runner + fake GitHub + `pkg/githubingest` + CodeSignal; no API/worker binaries | part of `test-acceptance` / a dedicated thin-proof task | Reserved name; lands with Task 0.3 |
| HTTP contract acceptance | Public HTTP routes, controlled fakes, deterministic store/time | `test-acceptance-core` | Reserved name; lands once `coach-api` exists (Baseline) |
| Platform workflow acceptance | Compose API + worker + Postgres + Redis Streams + model stub + fake GitHub + fixture repo + external runner | `test-acceptance` (full workflow leg) | Reserved name; lands once API + worker exist (Baseline) |
| Queue-provider conformance | Black-box contract against real Redis Streams + LocalStack SQS | `test-queue-conformance` | Harness defined in Task 0.4; executable legs land with Baseline Task 3a adapters |
| Operator smoke | Narrow credential-free Compose submission/poll path | a `platform-smoke`-style task | Reserved name; lands once smoke path exists (Baseline) |
| Native model validation | Operator-run Compose/native llama.cpp validation, schema-focused | a `platform-llm-validate`-style task | Reserved name; lands once Baseline's model path exists |

A lower layer passing is never evidence that a higher layer has run — each layer proves a different boundary, and `test-acceptance` (once it exists) must not silently omit a leg whose consumer (API, worker, queue adapter) is actually present.

### `mise run test-acceptance-fast`

```toml
[tasks.test-acceptance-fast]
run = "go test -race ./... -run Acceptance"
```

This runs every Go test function in the repo matching the substring `Acceptance` — currently `TestCodeSignalAcceptance`, `TestGitHubIngestAcceptance`, `TestCodeSignalReportAcceptance`, `TestSemanticsAcceptance`, `TestCoachAcceptance`, and `TestAcceptanceHarnessAcceptance` — without hardcoding package paths, so a future `*_test.go` naming a `TestXxxAcceptance` function is picked up automatically. These suites already run as part of `mise run test` (`go test -race ./...`); `test-acceptance-fast` doesn't add coverage, it names and scopes this layer explicitly per the taxonomy above. It is intentionally **not** added to `mise run ci`'s task list — Feature Zero's CI scope (gofmt/vet/tidy/test/examples/js-ci) is unchanged; `test-acceptance-fast` is a new, separately invocable task.

## 2. No-pull/offline Compose preflight behavior (specified here, implemented by Task 0.3)

This section documents a binding policy for Task 0.3's Compose runner; **no Compose files exist yet in this repository, and Task 0.1 does not implement this behavior** — it specifies the contract so Task 0.3 has no room to invent a different one.

Per the spec's Binding Offline Contract: "Compose tasks shall not implicitly pull images: they shall run in an offline/no-pull mode and fail with an actionable message naming a missing local image and the documented pre-acquisition step."

Concretely, once Task 0.3 lands, its Compose-driving `mise` task(s) must:

1. **Never implicitly pull.** Invoke Compose in a mode that treats a missing local image as an error rather than a trigger to fetch it (e.g. `docker compose --pull never` / `pull_policy: never` on each service, not the default `missing`/`always` behavior).
2. **Fail before any container starts**, not partway through the run — a missing image must be caught at preflight, before fake GitHub, Postgres, Redis, or the test runner container comes up.
3. **Name the specific missing image** in the failure message (e.g. `coach/fake-github:0.1.0`), not a generic "compose up failed."
4. **State the documented pre-acquisition step** the operator must have already run once, while online, before going offline — the eventual phrasing Task 0.3 will document is expected to be a `docker pull <image>` (or `docker compose pull`) step run against the pinned tag/digest, performed once before disconnecting from the network. This exact command and image list is Task 0.3's deliverable, not Task 0.1's; this doc records the requirement, not the final wording.

This complements, but is distinct from, the separate no-egress requirement (the Compose topology disables outbound network access for the test runner and application services and uses only the internal Compose network) — no-pull is about images, no-egress is about runtime traffic. Task 0.3 must satisfy both.

## 3. Golden fixture versioning and normalization rules

Future golden/versioned report fixtures (Task 0.3 onward — the thin Compose proof's report fixture, and later Task 0.4's deterministic/agent-provenance report fixtures) must follow these conventions so repeated acceptance runs stay reproducible despite intentionally-random generated credentials:

- **Every golden fixture embeds an explicit schema/version identifier field.** Per the spec's Story 0.4 acceptance criterion, this lets additive report evolution ("prove additive report evolution without changing prior report-version fixtures") add new fields or report versions without invalidating or silently reinterpreting an older golden.
- **Generated/non-deterministic values must never appear literally in a golden fixture.** This includes fixture tokens from `acceptanceharness.GenerateFixtureToken`, freshly-generated RSA keys from `acceptanceharness.GenerateRSAPrivateKeyPEM`, timestamps, and request IDs. Before comparison, normalize each such value to a fixed placeholder (e.g. `<GENERATED_TOKEN>`, `<GENERATED_TIMESTAMP>`) so that two runs using fresh generated credentials produce byte-identical goldens except for those explicitly normalized fields. This is the spec's binding contract: "repeated acceptance runs shall produce deterministic protocol observations and versioned/golden report fixtures, except for explicitly normalized generated identifiers."
- **`GenerateFixtureToken`'s caller-supplied prefix is designed for exactly this normalization.** A caller passes a semantically meaningful prefix (e.g. `"test-oauth-"`, `"test-installation-"`); a normalization pass can pattern-match on that stable prefix and redact only the random hex suffix (e.g. `test-oauth-<GENERATED_SUFFIX>`), leaving the prefix itself legible in the golden so a reader can still tell what kind of credential is represented.
- **`Clock`/`FakeClock` (`internal/acceptanceharness/clock.go`) are the seam later timestamp normalization and determinism work builds on.** Any report or protocol observation that includes a timestamp should be produced under a `FakeClock` with an explicit, fixed `start` time rather than `RealClock`, so the *un-normalized* value is already deterministic across runs; normalization to a placeholder is then a belt-and-suspenders step for defense in depth, not the sole source of determinism.

## 4. What `internal/acceptanceharness` provides

`internal/acceptanceharness` is the shared package later Feature Zero tasks (0.2's fake GitHub service, 0.3's Compose runner, 0.4's queue/agent-loop harnesses) build on. It is fully implemented and tested by prior tasks; this doc is where a reader should look before adding new acceptance-test infrastructure elsewhere in the repo.

- **Ambient-credential guard** (`guard.go`): `AmbientCredentialVars` (known GitHub/AWS ambient-credential env-var names), `ScanEnviron`/`ScanProcessEnv` (pure scan returning `CredentialGuardResult{Found []string}`, with `.Rejected() bool`), and `ScrubProcessEnv()` (unsets any found ambient-credential variable and returns what it scrubbed). Use this before an acceptance process creates an HTTP client or starts Compose services, per the spec's Story 0.1.
- **No-egress transport** (`guard.go`): `GuardedTransport`, built via `NewGuardedTransport(allowedHosts []string, fake http.RoundTripper) *GuardedTransport`, an `http.RoundTripper` that rejects any request to a non-allowlisted host *before* dialing (so a blocked request never reaches the real network) and records the blocked destination via `BlockedRequests() []string`. Wire this into any HTTP client an acceptance test builds so an accidental public request (e.g. to `api.github.com`) is observable and failing rather than merely discouraged.
- **Controlled clock** (`clock.go`): the `Clock` interface (`Now() time.Time`, `After(d time.Duration) <-chan time.Time`), `RealClock{}` for production code, and `FakeClock` (`NewFakeClock(start time.Time) *FakeClock`, `.Advance(d time.Duration)`) for tests that need deterministic heartbeat/timeout/reconciliation behavior without `time.Sleep` polling.
- **Test credential generation** (`testcreds.go`): `GenerateRSAPrivateKeyPEM(tb testing.TB) []byte` (a fresh, PKCS#1-PEM-encoded RSA key shaped like a GitHub App private key) and `GenerateFixtureToken(tb testing.TB, prefix string) string` (a `crypto/rand`-backed, prefixed synthetic token). Both fail the test immediately via `tb.Fatal` on error, and neither ever touches the network or a real credential.

All of the above is exercised by `internal/acceptanceharness/acceptance_suite_test.go` (`TestAcceptanceHarnessAcceptance`), following this repo's per-package acceptance-suite convention (e.g. `pkg/githubingest/acceptance_suite_test.go`).
