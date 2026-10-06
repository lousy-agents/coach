# ADR-007: Layered Ports-and-Adapters Architecture and Responsibility-Named Source Files

| Field | Value |
| --- | --- |
| Status | Accepted |
| Date | 2026-10-05 |
| Deciders | Repository maintainers |
| Supersedes | The ordinal `_partN` / `_body` / `_sigfixN` file split introduced while clearing self-scan findings |

## Context

`coach codesignal` scans this repository with the same thresholds it applies to customers: a file whose branch points (`if`, `for`, `switch`, type switch, `select`) reach 12 raises `complexity.branch_density`; a file with two or more pointer-returning functions, or two or more `New*` constructors, raises a density signal; a function nested 4 deep (function literals included) raises `complexity.max_nesting_depth`; and a function scoring 15 raises cognitive complexity. Clearing those findings across the repository split 747 Go files into ordinal fragments (`main_part2.go` … `main_part10.go`, `*_test_body_test.go`, `*_sigfix1_test.go`) and extracted nested test blocks into helpers named by position (`body_mainPart2Test_44`, `sigstartRecordingProxyListener39957725`).

The thresholds were met, but the split followed byte budgets rather than seams. One type's methods were scattered across three files, a file name no longer said what the file held, and a reader had to open every fragment to find a function. That is worse for humans and agents than the dense files it replaced: an agent locates code by file and package name, and an ordinal name carries none.

The density gate is per file and is the correct pressure. The missing piece was an architectural rule that says *where* a responsibility goes once a file is too dense to hold it.

## Options considered

All four options are layered; they differ in which way the dependencies between layers point, and that decides how much of the system can be tested without real I/O.

| Option | Shape | Testability | Verdict |
| --- | --- | --- | --- |
| Strict n-tier | presentation → business → data access; each tier calls only the one below it, concretely | Business logic imports the data tier, so a use-case test needs a real git repo, Postgres, or GitHub, or a mock of a concrete type. Test cost grows with every adapter | Feasible, least testable |
| Relaxed n-tier | as above, but a tier may skip down | Same coupling, plus presentation reaching data directly | Rejected |
| Vertical slices only | one package per feature, each owning its own I/O | Slices are small, but each slice mixes parsing, policy, and process execution, so the same coupling recurs inside each one | Rejected as the primary rule; kept as the grouping rule *within* a layer |
| Layered ports and adapters (dependency-inverted n-tier, "hexagonal") | domain ← application ← adapters; the application layer owns the interfaces (ports) adapters implement | Domain is pure (table tests over values); use cases run against in-memory fakes of their ports; each adapter is tested once against its real dependency. No test needs more than one real dependency | Chosen |

A layered architecture is feasible here because the repository already has the inner rings in place: `pkg/semantics` and `pkg/codesignal` are pure functions over bytes and values, `pkg/domain` holds the shared project facts, and the platform side already uses application-owned ports (`TaskQueue`, ADR-006; `CredentialResolver`, ADR-002; the `js/semantics` `Backend` seam). The work is to make the remaining packages follow the same direction and to name files and packages by the responsibility they hold.

## Decision

Organize Go code as **layered ports and adapters**: four layers with dependencies pointing inward only, grouped into capability packages within each layer.

```mermaid
flowchart LR
    Entry["Entry / driving adapters<br/>cmd/*, HTTP handlers, CLI flag parsing, renderers"]
    App["Application<br/>use cases + the ports they own"]
    Domain["Domain<br/>pure types and rules"]
    Infra["Driven adapters<br/>git, mise/npm/node probes, GitHub, Postgres, Redis, SQS, model gateway"]
    Entry --> App
    App --> Domain
    Infra -->|implements ports of| App
    Infra --> Domain
    Entry -.->|wires| Infra
```

### Layer map

The direction is enforced at two grains.

**Rings across the module.** `project.json` is the repository's own project-analysis policy. It places top-level trees in four rings and forbids every outward import between them, and `coach codesignal --baseline --project-config project.json` reports any crossing as `architecture.layer_violation`. It reports none.

| Ring (`project.json`) | Trees |
| --- | --- |
| entities | `pkg/domain`, `pkg/semantics` |
| usecases | `pkg/codesignal` |
| adapters | `pkg/githubingest`, `pkg/projectmodel`, `internal/codesignalcli`, `internal/coachapi`, `internal/jsbridge`, `internal/projectbridge` |
| app | `cmd/*` |

**Layers inside the application shells.** `internal/codesignalcli` (the CLI) and `internal/coachapi` (the platform API and worker) each sit in the adapters ring as a whole, because each wraps the inner rings for one delivery mechanism. Inside each shell, the same four-layer shape repeats at package grain:

| Layer | `internal/codesignalcli/…` | `internal/coachapi/…` |
| --- | --- | --- |
| Model | `projectreadiness` (readiness result), `projectconfig` (policy schema) | root (job, report, API contract, `JobStore` / `WorkerJobStore` ports) |
| Use case | root (scans, `ProjectBackend` port), `projectcheck`, `tssetup`, `configauthoring`, `sourcescope` | `baseline`, `worker` |
| Driven adapter | `gitrepo`, `revisionfs`, `tstoolchain`, `pkgmanager`, `subprocess`, `terminal`, `prompt` | `store/memory`, `store/postgres`, `queue/redisstream`, `queue/sqs` |
| Entry | `render` (wired from `cmd/coach`) | `httpapi` (wired from `cmd/coach-api`) |

`internal/codesignalcli/layering_test.go` pins the CLI shell's graph, so a new import edge between its packages fails a test rather than eroding silently. The domain libraries keep their I/O in private adapter packages: `pkg/semantics/internal/engine` (Tree-sitter) and `pkg/projectmodel/internal/tssidecar` and `ssaload`.

Where the migration stopped short of the rule, and why:

- The CLI use cases (`projectcheck`, `tssetup`, `configauthoring`, root `codesignalcli`) import `gitrepo` directly, and `projectcheck`/`tssetup` call `tstoolchain` and `pkgmanager` probes. Each git read goes through a function-typed seam owned by its consumer, but the toolchain probes are still package variables that tests override; replacing them with constructor-injected ports means reshaping the use-case signatures and is the next step.
- `internal/coachapi/baseline` constructs its local smoke-fixture tree source itself, because `ScanConfig.SmokeFixturePath` selects it; injecting the source would change worker configuration.
- `internal/rubrics` uses `internal/modelgateway`'s `Gateway` interface rather than declaring its own port.
- `pkg/projectmodel`'s exported entry points wire their default adapters (`tssidecar`, `ssaload`) because their signatures have no injection point and are public API.

The existing dependency rule still holds: `pkg/semantics` and `pkg/githubingest` never import each other.

### Rules

1. **Dependencies point inward.** A use case receives its I/O through an interface it declares (a port), not by constructing an adapter. Entry code constructs adapters and passes them in.
2. **A port lives with its consumer.** Declare the interface in the package that calls it, sized to what that caller uses. Adapters satisfy it implicitly.
3. **Group by capability inside a layer.** A package is named for what it does (`readiness`, `tstoolchain`, `gitrepo`, `store/postgres`), not for the layer (`services`, `utils`, `helpers`).
4. **One file, one named responsibility.** A file name states what the file holds: `flags_validate.go`, `catfile_reader.go`, `store_postgres_findings.go`. A type's methods live in the type's file unless a density gate forbids it; then the extra method or constructor gets its own file sharing the type's stem and named for what it does (`reader.go`, `reader_from_token.go`). Ordinal suffixes (`_part2`, `_body`, `_sigfix1`) are not responsibilities and are rejected by `mise run source-layout-check`.
5. **When the density gate fires, split along a seam.** Splitting a file means naming the two responsibilities it held. When a family of files in one package grows a shared prefix (`project_ts_compiler_*`), that prefix is a capability and becomes a package at the right layer.
6. **Tests mirror what they test.** A unit test file shares its production file's stem (`flags_validate.go` ↔ `flags_validate_test.go`). An acceptance spec is named for the behavior it specifies (`*_acceptance_test.go`); shared spec fixtures go in `*_fixtures_test.go` or `*_helpers_test.go`, never in a name the acceptance-style guard treats as a spec.
7. **Extracted helpers are named for their behavior.** A test helper pulled out to flatten nesting says what it asserts or builds (`expectSetupNeverContinues`, `serveRecordedProxyLines`), never where it came from (`body_mainPart2Test_44`) or a hash.

## Consequences

- A reader finds code by package and file name; no file needs to be opened to learn that it is not the one wanted.
- Use cases are testable with fakes of the ports they declare. Adapters carry their own tests against the real dependency (a temp git repo, Postgres in Compose, a recorded HTTP server).
- More packages, and some identifiers become exported to cross a package boundary. That is the price of a boundary the compiler enforces instead of one a file name implies.
- The density thresholds are unchanged. A future finding is resolved by naming a seam, not by appending a fragment, and `source-layout-check` makes the fragment route fail CI.
