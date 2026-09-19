# SOLID diagnostics

Load this before step 4c. Diagnose first, transform second. The
Structure rules in `tidy-rules.md` are remedies; this file decides
whether a remedy is warranted, which one, and how to prove it.

## Rules of engagement

- Diagnose only production files in this pass's work set. Read out-of-scope callers,
  tests, and git history as evidence; edit them only for mechanical
  call-sites.
- Function length is not a finding. A `switch` is not a finding. An
  interface with one implementation is not a finding by itself. Each
  finding needs evidence of an independent reason to change, a broken
  caller expectation, or a dependency that points the wrong way.
- Speculative vs justified: an abstraction with one implementation,
  no second use in the tree, and no test that exercises the seam is
  speculative — remove it under the Structure rules. A boundary with
  two or more concrete consumers or variations already in the tree,
  or a test that needs the seam, is justified — keep it; only narrow
  or move it.
- Do not introduce inheritance or class hierarchies into a repository
  that does not use them. Go interfaces and TypeScript structural
  types (`Pick`, function parameters) are the boundary vehicles.
- Record every accepted finding **before** any edit, in the state
  file's `## Structure findings` section, one line:

  `<path>:<line> → <principle> → <evidence> → <consequence> → <proposed refactoring> → <validation> → apply | defer (<reason>)`

- `apply` only when the remedy is an extract, rename, narrow, collapse,
  or move that stays in scope and preserves behavior. Anything else is
  `defer`: keep the line, put a one-line recommendation under
  `questions`, and add a pending ruling when the ruling list applies.
  A deferred finding never disappears from the record.
- `apply` never changes a public contract: an exported function,
  method, or interface keeps its exact signature. Narrowing an
  exported parameter's interface type is not compatible even when
  every existing argument satisfies the subset. Go needs identical
  parameter types wherever the function is held as a value
  (`var cb func(Wide) int = Report` stops compiling once `Report`
  takes `Getter`) and wherever the method satisfies a consumer-owned
  interface; TypeScript's parameter contravariance accepts those two
  uses, but the published declaration still changes, so the rule is
  the same. Keep the exported signature and delegate to an unexported
  helper that takes the narrow type; the tests exercise the helper.
  No helper worth extracting → `defer` (public API). Replacing a
  parameter with a different kind (a path with an interface, a value
  with a constructor), adding or removing parameters, or removing
  methods from an exported interface is the same `defer`, even when
  the in-scope tests still compile.
- Closed world: a signature may change only where **resolution
  itself** stops anything outside the searched tree from reaching the
  symbol. Go: `package main` — nothing can import it at all. Go
  `internal/` is **not** that, and a whole-module search does not
  make it one: in module mode the compiler admits any importer whose
  **import path** is under the internal parent's prefix, and that
  includes *other modules*. `example.com/acme/worker` is free to
  import `example.com/acme/internal/report`, so narrowing an exported
  parameter there leaves the producer's own tests green on an
  uncached run while the sibling module fails to compile
  (`cannot use report.Report (value of type func(r report.getter)
  int) as func(report.Wide) int value`). `internal/` is a closed
  world only when the search covers every module under that prefix,
  which one checkout rarely establishes — otherwise keep the
  signature, delegate to an unexported helper, or defer.
  TypeScript: an `exports` map that does not expose the
  module; verify the refusal
  (`ERR_PACKAGE_PATH_NOT_EXPORTED`) rather than assuming it, and
  search the whole repository, because a workspace sibling importing
  `../pkg/lib/report.js` by relative path never consults that map.
  `private: true` and "the barrel does not re-export it" are **not**
  closed worlds: `private` only blocks publishing, and with no
  `exports` map `require("pkg/lib/report.js")` resolves from any
  linked or workspace install. Both leave the producer's own tests
  green while a downstream call breaks. Inside a verified boundary
  the consumer surface is complete only when Grep over that whole
  tree lists every reference; then change them all — calls, function
  values, interface satisfactions — in the same step. Anything less
  (an unverified boundary, a consumer set you cannot enumerate, a
  reference outside the work set that is not a mechanical call-site)
  keeps the signature and records a public-API `defer`.
- Record every confirmed finding at a location, even when another
  principle already has one there; say when one remedy serves both.
- Narrate one line when done: `Diagnose  <n> findings, <n> apply, <n> defer`.

## Single responsibility

- **Signals:** one type or function that persistence, formatting, and
  policy all reach into; unrelated changes landing in the same file
  across the history; a test that needs two unrelated fixtures for
  one unit.
- **Evidence:** `git log --format=%s -- <path>` for independent
  reasons the file changed; callers grouped by concern; the fixtures
  its tests build.
- **Confirmed when:** two concerns would change for different reasons
  and different requesters, and each has its own callers or tests.
- **Not a violation:** a long function whose steps all serve one
  reason to change. Extract it for readability under the Structure
  rules; do not record it as SRP.
- **Example (Go):** `SaveInvoice(inv Invoice, db *sql.DB)` computes
  tax and writes rows; tax policy and storage change independently.
  Remedy: extract `taxFor(inv)` as a pure function, leave the write
  where it is. **(TS):** `formatAndPost(order)` renders HTML and calls
  `fetch`; extract `render(order)`.
- **Validation:** a characterization test on the observable result of
  each concern before the extraction; suite green vs baseline after.
- **Ruling when:** the split changes a serialized format or a public
  API.

## Open/closed

- **Signals:** the same `switch` / `if` chain over a kind repeated in
  several functions; a comment saying "add the new case here too";
  history, when it exists, showing each new kind edited all of them.
- **Evidence:** grep the kind's constants and read every site that
  enumerates them for what each site *decides*. A repeated
  enumeration is a candidate, not a finding. `git log -p -- <path>`
  corroborates when history exists; a squashed or single-commit
  branch has none, and that absence is not evidence against.
- **Confirmed when:** two or more sites make the same decision over
  the set — they select the same per-kind behavior (a renderer, a
  handler, a parser) or re-derive the same per-kind fact — so a kind
  added at one site is silently wrong at the others, and one lookup
  in the place that owns the kinds would make a new kind a one-place
  addition. The tree is enough; do not wait for history.
- **Not a violation:** one `switch` in one place; a variation that has
  happened once; independent operations over a deliberately closed
  set — a wire encoder, a display label, and an audit class each
  switching exhaustively over the same protocol enum decide three
  different things, a new variant legitimately needs all three
  decided, and one table would couple code that changes for
  different reasons. Colocated property switches over one enum may
  collapse into a table under the Structure rules when that reads
  better; record that as a collapse, not as OCP. Never add an
  extension point for a variation that has not occurred.
- **Example (Go):** `Export`, `Preview`, and `Supported` each switch
  on `report.Kind` to pick the same renderer. Remedy: collapse to one
  lookup, `renderers[kind]`, in the place that already knows the
  kinds — a move plus a collapse, not a new interface. **(TS):** a
  discriminated union handled by one `switch` in one module is fine;
  the same dispatch copied into three modules is the finding.
- **Validation:** the per-kind tests still pass; one test proves an
  unknown kind fails the same way it did before; the diff touches
  only the sites that made the same decision.
- **Ruling when:** the extension point would become part of a public
  API.

## Liskov substitution

- **Signals:** an implementation that panics or throws "not
  supported", ignores an argument the interface documents, needs
  setup the interface does not mention, or returns success without
  doing the work.
- **Evidence:** every caller's expectation of each interface method
  (what it does with the result and the error), compared with each
  implementation's preconditions, guarantees, and side effects.
- **Confirmed when:** a caller written against the interface would
  behave incorrectly with one implementation: a stronger
  precondition, a weaker guarantee, an unsupported operation, or a
  side effect the others lack.
- **Not a violation:** an implementation that is slower, logs more, or
  returns a narrower error than documented.
- **Example (Go):** `Store.Delete(id)` returns nil for a missing id in
  the in-memory implementation and `ErrNotFound` in the SQL one; a
  caller retries on nil. Remedy: make both return the documented
  error, and hold it with one contract test run against every
  implementation (extract the test, not a type). **(TS):** a
  `Notifier.send()` that resolves without sending in production is
  the finding; a test double that does so by design is not.
- **Validation:** one contract test per interface method, run against
  all implementations in scope.
- **Ruling when:** the fix changes the documented contract, or the
  odd implementation's behavior may be intentional and the tests and
  PR description are silent.

## Interface segregation

- **Signals:** consumers that stub or panic in methods they never
  call; test doubles that implement ten methods to exercise one; an
  interface named after a type rather than a capability.
- **Evidence:** for each consumer, the methods it actually calls; for
  each interface, its method count against the widest consumer's use.
- **Confirmed when:** at least one consumer depends on methods it never
  calls, and those methods force stubs or coupling to a wider
  implementation.
- **Not a violation:** a broad implementation consumed through a
  narrow dependency. Width of the type is fine; width of the
  dependency is the finding. Nor is a method the consumer skips
  because it bypasses that seam (it reads the environment or a file
  instead): that is the dependency-inversion finding below. Never
  narrow around a bypass; record the DIP finding and let its remedy
  decide the interface.
- **Example (Go):** `type Repo interface { Get; Put; Delete; List;
  Migrate }` consumed by a reader that calls only `Get`. Remedy:
  narrow — declare `type getter interface{ Get(...) }` beside the
  consumer and take that; the broad type still satisfies it. When the
  reader is exported, `func Report(r Repo)` keeps that signature and
  delegates to `func report(r getter)`, and the tests call `report`;
  changing `Report` to take `getter` breaks a consumer that holds it
  as a `func(Repo) …` value or satisfies `interface{ Report(Repo) }`
  with it, which the package's own tests never show. **(TS):** accept
  `Pick<Repo, 'get'>` at the consumer; an exported function keeps its
  declared parameter type the same way.
- **Validation:** the consumer's tests now use a one-method double
  (through the helper when the consumer is exported); suite green vs
  baseline; every exported signature in the file unchanged.
- **Ruling when:** a consumer of the interface is out of scope, the
  interface is exported from a published package and would lose
  methods, or the only remedy changes an exported signature.

## Dependency inversion

- **Signals:** business policy that imports a database driver, an HTTP
  client, a clock, or a filesystem directly; constructors that build
  their own infrastructure; policy tests that need a network or a
  real file.
- **Evidence:** the in-scope package's import graph (`go list -deps`,
  or the TS import lines); tests that are skipped or slow because of
  the dependency; a seam that already exists and is bypassed.
- **Confirmed when:** a policy decision cannot be tested without the
  infrastructure, and the infrastructure detail leaks into the
  policy's signature or behavior.
- **Not a violation:** an adapter whose whole job is the file,
  database, or HTTP format it speaks — a loader that takes a path and
  returns parsed records, a writer that takes an `io.Writer`, the
  package that imports the driver it adapts; a `main` that wires
  concrete types; a small program with one layer. Decide by
  responsibility and direction first: a temp-dir or `httptest` test
  proves the code is testable, not that policy and infrastructure are
  separated. Policy that opens and parses a vendor file, or speaks
  SQL or HTTP itself, and also decides something (a discount, an
  eligibility) is coupled to that storage even when it takes a path
  and has a temp-dir test: record the finding. Its remedy usually
  changes an exported signature → `defer` (public API) with the
  boundary named in the recommendation; never suppress a diagnosis
  because the fix is not yours to apply.
- **Example (Go):** `pricing.Quote` calls `sql.Open` itself. Remedy:
  take the narrow consumer-owned interface the policy actually needs
  (`type rateSource interface{ Rate(ctx, sku) }`). Moving `sql.Open`
  to the caller and passing `*sql.DB` fixes ownership while the
  policy still speaks SQL: record it as a partial step and keep the
  line open, never as the finding resolved. Do not invent a
  repository framework. **(TS):** a pricing module that calls `fetch`
  takes a `rates: (sku) => Promise<Rate>` parameter.
- **Validation:** the policy test runs with an in-memory rate source
  and no network or file; the adapter keeps its temp-dir or
  integration test.
- **Ruling when:** moving the wiring outward touches files far outside
  the scope list, or the boundary changes a public constructor or any
  other exported signature.

## Deferred findings across passes

Write the line with `defer` and the reason (a ruling name, `out of
scope`, `changes behavior`, or `public API`). Copy a one-line
recommendation to `questions`. Add the pending ruling when the reason
is on the ruling list. The next pass reads the Structure findings
section before diagnosing and does not re-diagnose a deferred finding
whose evidence has not changed.
