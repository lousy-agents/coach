# Go Test Patterns

Concrete, topic-organized Go testing patterns. Informed by patterns observed
in [`learn-go-with-tests`](https://github.com/quii/learn-go-with-tests), a community-maintained guide to TDD in Go, plus pragmatic conventions for existing codebases
with established handwritten test styles. Use whichever subsection matches
the task at hand; none of these require reproducing any particular book or
tutorial structure.

## Desired Defaults (Existing Codebases)

- Read nearby `*_test.go` files before choosing a pattern.
- Keep tests straight-line: arrange inputs and fakes, call the unit, inspect
  outputs or side effects.
- Make each test read as executable documentation for the behavior. A reader
  should understand the rule, important example, and expected outcome without
  reverse-engineering production internals.
- Prefer behavior names over implementation names in subtests: `rejects
  expired tokens`, `writes the invoice to the store`, or `returns 404 for
  missing players`, not `calls validateToken` or `handles branch 3`.
- Use local dummy types, spies, stubs, and small helpers before shared test
  utility packages.
- Use direct `if got != want { t.Fatalf(...) }` assertions when they match
  local style.
- Keep expected values visible in tests unless helper extraction clearly
  improves readability.
- Make assertion failures explain the contract: include the behavior,
  relevant input/state, and `got`/`want` or actual error details.
- Add table tests or subtests only when multiple cases share the same shape
  or failure names matter.
- Avoid adding assertion libraries or mocking frameworks unless the repo
  already uses them.

## Detecting the Repository's Acceptance-Test Convention

Unit tests and acceptance tests are different products with different
defaults. Unit tests default to the standard library. An acceptance suite —
the coverage that proves a feature or bug fix works at the public boundary —
takes whatever form the repository already mandates. Detect that form before
writing the test; do not infer it from this document.

Run these checks and combine the results. Check 1 alone is enough to stop when
it finds an existing acceptance suite. Check 2 is never enough on its own:
many modules pull in assertion helpers (especially `testify/assert` or
`testify/require`) for unit tests without adopting that library as an
acceptance harness.

```bash
# 1. Existing acceptance suites, by filename or directory
find . -name '*_test.go' | grep -Ei '(_acceptance_test\.go|/(acceptance|e2e|features|specs?)/)'

# 2. Suite-harness or spec-runner modules already in the dependency graph.
#    Match suite/spec packages, not bare assertion helpers: `testify` alone is
#    not a signal — only `testify/suite` (or another runner below) counts.
find . -name go.mod -exec grep -Ein \
  '(ginkgo|gomega|testify/suite|godog|goconvey|check\.v1)' {} +

# 3. A suite entrypoint — the bootstrap that hands control to a runner.
#    `func TestMain` on its own does NOT count: it is routine stdlib plumbing
#    for flag parsing, fixture setup, or container teardown. Treat it as an
#    entrypoint signal only when check 1 or check 2 also matched.
find . -name '*_test.go' -exec grep -En '(RunSpecs|suite\.Run|godog\.TestSuite|func TestMain)' {} +

# 4. Declared acceptance targets
find . \( -name Makefile -o -name '*.toml' -o -name 'Taskfile.y*ml' \
  -o -path '*/.github/workflows/*' \) -exec grep -Ein 'acceptance|e2e' {} +

# 5. Stated project policy
find . \( -name AGENTS.md -o -name CLAUDE.md -o -name CONTRIBUTING.md \) \
  -exec grep -Ein 'acceptance|e2e|spec runner|test (harness|framework|suite)' {} +
```

Every check searches the tree instead of naming files that may not exist, so a
repository without a `go.mod` at the root, a task runner, or agent docs simply
returns nothing. Empty output is a clean "no signal", not a tooling failure.

Reading the result:

- **A convention exists** (an acceptance suite file; a runner module from
  check 2 *plus* a matching entrypoint from check 1 or 3; or a written
  policy): write the acceptance test in that form, matching the existing
  suite's file layout, naming, and bootstrap. Do not introduce a second
  acceptance style alongside it.
- **No convention exists**: the acceptance form is standard-library `testing`,
  written the way the rest of the repository writes tests. Do NOT add a spec
  runner, BDD framework, or assertion library to a repository that does not
  already depend on one — that is a dependency decision owned by the module's
  maintainers, not something to settle inside a test. A `go.mod` that only
  lists unit-test assertion helpers is still "no convention".
- **Signals conflict** (e.g. a runner sits in `go.mod` but the package under
  test uses stdlib tables): follow the nearest existing acceptance suite, and
  state in your report which signal you followed and which you set aside.

### The Same Criterion in Two Acceptance Forms

The criterion does not change; only the vehicle does. Both versions below make
the rule legible from structure alone.

Standard library `testing` — the form to use when the repository has no
runner:

```go
func TestWithdrawAcceptance(t *testing.T) {
    t.Run("an account with insufficient funds", func(t *testing.T) {
        t.Run("rejects the withdrawal and leaves the balance unchanged", func(t *testing.T) {
            account := givenAccountWith(Money(20))

            err := whenWithdrawing(account, Money(25))

            thenErrorIs(t, err, ErrInsufficientFunds, "overdrafts must be rejected")
            thenBalanceIs(t, account, Money(20), "a rejected withdrawal must not move money")
        })
    })
}
```

A nested-block spec runner — use this form *only* when the repository already
runs one. The example below is written in Ginkgo/Gomega syntax; a testify
`suite`, a `godog` feature file, or an in-tree harness expresses the same
structure with its own vocabulary:

```go
var _ = Describe("Withdraw", func() {
    var account *Account

    When("the account has insufficient funds", func() {
        BeforeEach(func() { account = NewAccount(Money(20)) })

        It("rejects the withdrawal and leaves the balance unchanged", func() {
            err := account.Withdraw(Money(25))

            Expect(err).To(MatchError(ErrInsufficientFunds))
            Expect(account.Balance()).To(Equal(Money(20)))
        })
    })
})
```

The mapping is mechanical: the runner's outer blocks carry what `given...`
helpers and outer subtest names carry, and its innermost block carries what
the innermost subtest name and `then...` assertions carry. Whichever vehicle
the repository uses, the bar is identical — the structure names the criterion,
and the failure output names the behavior that broke.

## Starting Outside-In

For a new feature or a bug fix, the first failing test belongs at the public
boundary — the CLI invocation, the HTTP request, or the exported call a user
actually makes. Write it in whichever acceptance form the detection above
identified; the walkthrough uses standard-library `testing` because that is
the form for a repository with no runner.

1. **Write the boundary test first, calling the API you wish existed.** The
   test is that API's first consumer, so let it invent the seam instead of
   designing the seam up front. This one needs a rate source that production
   does not have yet — and nothing else, so invent nothing else:

   ```go
   func TestQuoteAcceptance(t *testing.T) {
       t.Run("prices an order in the customer's currency", func(t *testing.T) {
           quoter := NewQuoter(stubRates{"EUR": 0.9})

           got, err := quoter.Quote(Order{Subtotal: Money(100), Currency: "EUR"})

           if err != nil {
               t.Fatalf("quoting a EUR order should succeed: got err %v", err)
           }
           if want := Money(90); got != want {
               t.Fatalf("EUR order priced at the source's rate: got %v, want %v", got, want)
           }
       })
   }
   ```

2. **Let the test define the ports.** `stubRates` does not exist yet either;
   write it in the test file, and the interface it satisfies becomes the port
   production accepts through its constructor:

   ```go
   // In the test file — the fake the test just invented.
   type stubRates map[string]float64

   func (r stubRates) Rate(currency string) (float64, error) {
       rate, ok := r[currency]
       if !ok {
           return 0, fmt.Errorf("no rate for %s", currency)
       }
       return rate, nil
   }

   // In production — the port that fake forced, and the constructor that takes it.
   type RateSource interface {
       Rate(currency string) (float64, error)
   }

   func NewQuoter(rates RateSource) *Quoter // body comes in step 3
   ```

   If the arrangement is awkward to write — too many parameters, a hidden
   global, an unclear return — that is the API being awkward, not the test.
   Change the signature before writing production code.

3. **Make it pass with the smallest honest change.** A hard-coded conversion
   or a single-branch `Quote` is enough; the boundary test only has to go
   green.

4. **Drill inward.** Add unit tests for the pieces that carry real logic —
   rounding rules, unsupported-currency errors, missing-rate handling — and let
   those tests own the edge cases. The boundary test keeps proving the feature
   works end to end; it should not grow a case per rounding rule.

Inside-out remains the right start for a pure internal helper, an algorithmic
core, or a well-understood domain whose public API is already settled: write
the unit test directly and skip the boundary step.

## Executable Documentation and Diagnostic Assertions

Tests should document the behavior a caller relies on, not the mechanism the
production code currently uses. A useful test failure should let a human or
agent infer the intended production behavior from the failure output.

Use this shape:

```go
func TestWithdraw(t *testing.T) {
    t.Run("rejects withdrawals that would overdraw the account", func(t *testing.T) {
        account := NewAccount(Money(20))

        err := account.Withdraw(Money(25))

        if !errors.Is(err, ErrInsufficientFunds) {
            t.Fatalf("overdraft withdrawal should fail with ErrInsufficientFunds: got %v", err)
        }
        if got, want := account.Balance(), Money(20); got != want {
            t.Fatalf("failed overdraft should leave balance unchanged: got %v, want %v", got, want)
        }
    })
}
```

For tables, make case names read like the spec and include the case in
failure output:

```go
tests := []struct {
    name string
    role Role
    want []Permission
}{
    {"admin can manage users", Admin, []Permission{ManageUsers}},
    {"viewer can only read reports", Viewer, []Permission{ReadReports}},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got := PermissionsFor(tt.role)

        if !slices.Equal(got, tt.want) {
            t.Fatalf("permissions for role %s: got %v, want %v", tt.role, got, tt.want)
        }
    })
}
```

When comparing large structured values, prefer parsed representations or an
existing local diff helper. If the standard library is enough, compare
specific fields and make each failure name the contract being checked. If
the repository already uses a diff package, include the diff in the failure
message with behavior context.

## Acceptance Criteria as Given/When/Then Helpers

When a test exists to satisfy a specific acceptance criterion (from a spec,
issue, or review comment), prefer expressing that criterion through the
test's own structure rather than only through a comment. Extract small
`given`/`when`/`then` helpers, or use a descriptive subtest name, so the
criterion is legible from the code itself.

Before — the comment carries all the meaning, and the test body must be read
line-by-line to reconstruct the rule:

```go
// AC-1.8: FetchFile must re-check ctx.Err() after decoding and before
// returning, so a context canceled mid-decode is still honored.
func TestFetchFile_ChecksCancellationAfterDecode(t *testing.T) {
    ctx := &cancelAfterNCallsContext{Context: context.Background(), n: 1}
    client := newTestClient(t, canned200Response)

    _, _, err := client.FetchFile(ctx, someRef)

    if !errors.Is(err, context.Canceled) {
        t.Fatalf("got err %v, want errors.Is(err, context.Canceled)", err)
    }
}
```

After — `given`/`when`/`then` helpers make the criterion executable; the
comment becomes optional traceability metadata instead of the sole
explanation:

```go
// AC-1.8
func TestFetchFile_ChecksCancellationAfterDecode(t *testing.T) {
    ctx := givenContextCanceledAfter(1, "the decode step")
    client := givenClientReturning(t, canned200Response)

    _, _, err := whenFetchFile(ctx, client, someRef)

    thenErrorIs(t, err, context.Canceled, "FetchFile must honor cancellation observed after decoding")
}

// givenContextCanceledAfter returns a context that reports nil from Err()
// for the first n calls, then context.Canceled, isolating exactly the check
// under test (labeled by where) from any earlier cancellation checks.
func givenContextCanceledAfter(n int, where string) context.Context { /* ... */ }

func givenClientReturning(t *testing.T, respond responseFunc) *FileClient {
    t.Helper()
    return newTestClient(t, respond)
}

func whenFetchFile(ctx context.Context, c *FileClient, ref FileRef) ([]byte, FileMetadata, error) {
    return c.FetchFile(ctx, ref)
}

func thenErrorIs(t *testing.T, err, target error, why string) {
    t.Helper()
    if !errors.Is(err, target) {
        t.Fatalf("%s: got err %v, want errors.Is(err, %v)", why, err, target)
    }
}
```

Guidance for applying this:

- Reach for this pattern when a comment is doing work that naming and
  structure should do instead — typically when the comment restates a rule
  that the test body doesn't otherwise make obvious.
- Keep `given`/`when`/`then` helpers small and single-purpose. For helpers
  that accept `*testing.T`/`testing.TB` — typically `then`/`assert`
  helpers — call `t.Helper()` so failures point at the calling test, not the
  helper; pure `given`/`when` builders that don't take `t` don't need it.
- A short traceability comment (an acceptance-criteria ID, issue link) above
  the test is still fine — it should stop being the *only* place the
  behavior is explained.
- Don't force this shape on every test. A short, already-legible
  straight-line test does not need extraction; only convert when the comment
  is compensating for unclear structure.

## Implementation-Coupling Anti-Patterns

Avoid tests that cement current production structure instead of behavior:

- Testing unexported helper functions only because the exported behavior
  currently delegates to them. Test the exported behavior unless the helper
  is itself a stable package contract.
- Verifying every mock call, call order, or intermediate collaborator when
  the caller only cares about final output, persisted effects, emitted
  events, or external requests.
- Rebuilding the production algorithm in the test to compute `want`; this
  lets the same bug exist in both places. Use concrete examples,
  independently known fixtures, or properties instead.
- Asserting exact SQL strings, JSON field order, map iteration order,
  timestamps, generated IDs, log wording, or full rendered documents unless
  that exact representation is the documented contract.
- Asserting private struct fields when a public method, returned value,
  stored record, emitted event, or fake boundary can express the behavior.
- Adding broad snapshot or golden-file tests without focused assertions for
  the behavior that matters. Fixtures should clarify expected output, not
  hide a large opaque blob.
- Mirroring production branching in the test (`if input.X { want = ... }`)
  instead of naming separate examples.
- Making tests pass by exposing production internals only for tests. Prefer
  testing through public behavior or extracting a real boundary.

Interaction assertions are valid when the interaction is the behavior: a CLI
must invoke a command with specific arguments, a service must write a record,
a handler must call a dependency with the authenticated user, or a goroutine
must cancel work. Keep those assertions at the boundary contract, not at
incidental helper calls.

## Pure Functions and Foundational Progression

Use for small language concepts and pure behavior with no external
dependencies.

1. Write one direct test for one visible behavior.
2. Hard-code the simplest result.
3. Add a second test that forces parameterization.
4. Introduce subtests and helpers once cases repeat.
5. Add error cases before broadening the API.
6. Move from arrays to slices once inputs vary in length; use `slices.Equal`
   (Go 1.21+) for comparisons; cover empty-slice edge cases.
7. Move from free functions to methods, then interfaces, once behavior needs
   state or substitution; use table tests once multiple types implement the
   same interface.
8. Use pointer receivers for mutation; use sentinel or custom errors for
   invalid state transitions; write small assertion helpers
   (`assertBalance`, `assertNoError`, `assertError`) once repeated across
   tests.
9. For map-backed logic, cover lookup, add, update, and delete, using
   subtests for known, unknown, existing, and missing keys.

## HTTP Handlers and JSON APIs

- Construct handlers with explicit dependencies, such as
  `NewHandler(service Service)` or `NewServer(store Store, logger Logger)`.
- Use `httptest.NewRequest`/`httptest.ResponseRecorder` for handler-level
  unit tests; use `httptest.NewServer` for wiring-level/integration tests.
- Test status codes, headers, response bodies, decoded JSON
  (`json.NewEncoder`/`json.NewDecoder`), and service or store interactions
  as appropriate.
- Keep handlers thin: parse the request, call business logic, encode the
  response.
- Use local dummy services or stores for handler tests; reserve
  integration-style tests for when routing, middleware, or real wiring is
  the behavior under test.
- Isolate metrics registries, clocks, loggers, or package-level state per
  test, preferably through constructors.
- Use `http.NewServeMux` composition and small route-specific tests rather
  than one large end-to-end test per route.

## File Parsing, Rendering, and CLI/Process/Filesystem Boundaries

- Put command execution, environment access, filesystem access, stdin,
  stdout, and stderr behind constructor-injected interfaces, function
  fields, or standard-library abstractions.
- Test process boundaries with fake executors; assert command name,
  arguments, call count, output handling, and error behavior.
- Test filesystem behavior with `fs.FS`, `fstest.MapFS`, temp directories,
  `bytes.Buffer`, or injected readers/writers instead of hard-coded host
  paths.
- Split package logic from composition roots (e.g. `cmd/.../main.go`); wire
  the same store/service through multiple entrypoints (CLI, web server)
  where applicable.
- For rendering (HTML, Markdown, templates, SVG), assert against parsed
  output or approved fixtures rather than fragile full-string matches;
  render through an `io.Writer` and use embedded templates where useful.
- For structured input, parse from `fs.FS` rather than hard-coded OS paths.
- Keep command and filesystem tests focused on the boundary contract.

## Time-Based and Orchestration Testing

- For game/orchestration-style logic with blind alerts, timers, or input
  validation, separate orchestration from time-sensitive triggers.
- Use retry/timeout helpers and fake clocks instead of `time.Sleep` in
  tests, unless the behavior under test is explicitly timing-based.
- Assert time-sensitive tests deterministically: inject a clock or ticker
  abstraction rather than depending on wall-clock timing.

## WebSocket and Bidirectional Integration Testing

- Test live websocket behavior with real client/server integration tests
  where the wiring itself is the behavior under test.
- Assert bidirectional message delivery (e.g. alert delivery) by exercising
  both send and receive paths against a running test server.

## Dependency Injection, Mocking, and Spies

- Inject `io.Writer` (or similar boundaries) to test output without
  touching real stdout/files.
- Inject interfaces or function fields (e.g. a `Sleeper`) instead of
  calling concrete implementations directly, so tests can substitute fakes.
- Use spies to assert ordering and interaction between collaborators, not
  just final state.
- Prefer constructors such as `NewService(store Store, clock Clock)` or
  `NewHandler(service Service)` for code with external dependencies.
- Keep concrete dependencies in composition roots; accept interfaces or
  function types at behavior boundaries.
- Store injected dependencies on structs instead of reading globals,
  opening files, creating clients, or calling process APIs inside business
  logic.
- In tests, pass local fakes, spies, stubs, buffers, fake filesystems, fake
  clocks, or fake executors through constructors.
- Keep zero-value friendliness where practical, but prefer explicit
  dependency wiring for code that touches external state.
- Avoid service locators, mutable package-level state, and hidden singleton
  clients.

## Package Fitness and the Dependency Rule

A package's imports decide how testable it is before a test is written: a
package that imports no IO needs no fakes, no `httptest` server, and no temp
directory. Keep calculation and decision packages pure, keep the adapters —
HTTP clients, database access, parsers, filesystem readers — in packages at
the edge, and let the composition root wire the two together.

Declare interfaces at the use site. The package that *consumes* a dependency
declares the small interface it needs next to the code that calls it (e.g.
`type RateSource interface { Rate(string) (float64, error) }`); the package
that *provides* it returns concrete types. The dependency then points from
the adapter toward the pure package, and each test's fake stays local to the
test that needs it.

Check this rather than trusting it, with two commands. The first walks the
module's own packages the target reaches and prints the packages *they*
import directly, then greps for IO. The second drops the module's own packages
and reports the third-party ones that remain. Each is a single `go list` — no
`xargs` second hop.

```bash
# stdlib IO reached through your own packages
go list -deps -f '{{if and .Module .Module.Main}}{{range .Imports}}{{.}}{{"\n"}}{{end}}{{end}}' ./pricing |
  sort -u | grep -E '^(os|net|log|database/sql)(/|$)'
# third-party dependencies
go list -deps -f '{{if and .Module (not .Module.Main)}}{{.ImportPath}}{{end}}' ./pricing
```

Keep the two separate. A single `go list -deps` piped into one IO grep also
matches the stdlib's own internals: `fmt` and `encoding/json` both depend on
`os`, so that shorter form reports `os` for any package whose only sin is
calling `fmt.Errorf` — a false positive on exactly the pure domain packages
this section tells you to build.

Two details in that form are load-bearing. The `-f` template asks `go list`
which module each dependency belongs to instead of string-matching the module
path, so a multi-module `go.work` (where `go list -m` prints several lines and
silently corrupts the pattern) still classifies correctly. And the IO check
prints each main-module package's *direct* `.Imports` inside that same
invocation — it never shells out to a second `go list` via `xargs`. A two-step
`go list | xargs go list` form is how a typo'd path or undownloaded dependency
used to print a confident, entirely fabricated impurity report (bare `xargs`
falls back to the package in the current directory).

A package that is still pure prints nothing from either command (grep exits 1
on no match):

```
$ <io-check>
$ echo $?
1
$ <third-party-check>
$ echo $?
1
```

The same package after someone reaches for `os` and `net/http` inside it:

```
$ <io-check>
net/http
os
```

A package that has picked up a third-party dependency:

```
$ <third-party-check>
github.com/acme/rates/rates
```

That second list is a review list, not a verdict. It is transitive like the
IO check, so a package whose own imports are entirely stdlib can still list a
third-party path it reaches through an internal sub-package that exists to own
exactly that dependency — a parser package wrapping a grammar library, say.
That is the shape this section is asking for: the dependency is quarantined
behind a seam instead of spread through the domain. Read the list and ask
whether each entry is quarantined or has leaked into the code that holds
decisions; only the second is a finding.

Notes on running them:

- The IO check is transitive across your own packages, which is the point: a
  pure package that depends on a helper package that opens files is not pure
  either. To find which import caused a hit, list a package's direct imports:
  `go list -f '{{join .Imports "\n"}}' ./pricing`.
- A third-party dependency shows up as its full import path
  (`github.com/acme/rates/rates`). Stdlib packages belong to no module, so
  `go list`'s module metadata excludes them from both checks automatically.
- Extend the IO alternation with whatever else must stay out of the
  package (`os/exec`, `database/sql`, a config or logging package of your
  own).
- `go list` needs the module graph to resolve. If dependencies are not
  downloaded it errors instead of printing packages — that is a check that
  did not run, not a package that passed.
- The symptom that precedes a failing check: a unit test that needs three
  fakes to reach one calculation. Moving that calculation into a package
  with no IO imports usually deletes the fakes outright.

## Testing Goroutines and Concurrency

Concurrent code needs its own deliberate test strategy — non-deterministic
scheduling means a naive test can pass most of the time and still hide a
data race.

1. **Benchmark first.** Before introducing goroutines, write a benchmark
   (`func BenchmarkX(b *testing.B)`, run with `go test -bench=.`) against
   the sequential implementation to justify the change and to measure the
   improvement afterward.
2. **Expect the naive concurrent version to fail or race.** Turning a loop
   body into `go func() { ... }()` without coordination typically returns
   before goroutines finish (an empty/partial result), or — if multiple
   goroutines write to a shared map or slice — corrupts memory. Go maps are
   not safe for concurrent writes; the runtime can throw
   `fatal error: concurrent map writes`.
3. **Never "fix" this with `time.Sleep`.** Sleeping a fixed duration to
   "let goroutines finish" is not a real fix: it is slow, flaky under load,
   and does not prevent the underlying data race.
4. **Always run concurrent tests with the race detector:**
   `go test -race`. Treat any race detector failure as a real bug to fix,
   not a flaky test to retry.
5. **Coordinate with channels, not shared mutable state.** Send a small
   result struct (e.g. `type result struct { url string; ok bool }`) from
   each goroutine over a typed channel (`resultChannel := make(chan
   result)`), then receive exactly as many results as goroutines started
   in a single collecting loop:

   ```go
   resultChannel := make(chan result)

   for _, url := range urls {
       url := url // shadow: avoid capturing the shared loop variable (Go < 1.22)
       go func() {
           resultChannel <- result{url, wc(url)}
       }()
   }

   for i := 0; i < len(urls); i++ {
       r := <-resultChannel
       results[r.url] = r.ok
   }
   ```

   This keeps each `wc(url)` call concurrent while serializing the writes
   to shared state, eliminating the data race without sacrificing the
   speedup.
6. **Use `sync.WaitGroup`** when goroutines must complete before a test or
   function returns but there is no per-goroutine result to collect.
7. **Use `select` with `time.After` or a `context.Context` timeout** in any
   test that waits on a channel, so the test fails fast with a clear
   timeout error instead of hanging indefinitely if a goroutine never sends.
8. **Protect shared state with a mutex** (`sync.Mutex`/`sync.RWMutex`) only
   when channels are not a natural fit; prove the protection works with a
   test that exercises concurrent access (ideally under `-race`).
9. **Assert on the collaborator, not just the final value**, for
   cancellation: confirm the goroutine was told to stop (e.g. via a spy or
   a cancellation-aware fake), not only that some response eventually
   returned.

## Reflection, Property-Based Testing, and Generics

- For code that walks arbitrary structures, recursively walk
  `reflect.Value` across structs, pointers, slices, arrays, maps, channels,
  and functions; write table tests for representative shapes.
- Use `testing/quick` to add property/round-trip checks (e.g. encode then
  decode returns the original value) after concrete examples establish the
  expected behavior — properties complement examples, they don't replace
  them.
- Introduce generic type parameters only for real, repeated behavior across
  types; keep constraints narrow (`any` for containers, `comparable` for
  equality assertions or set-like behavior) and keep zero values
  predictable.
- For greedy/ordered algorithms (e.g. symbol-table-driven conversions), back
  them with table tests plus property tests for round-trip correctness.

## Business Logic

- Keep core decisions pure where possible and test them with direct inputs
  and expected outputs.
- Use constructor-injected collaborators such as repositories, clients,
  stores, clocks, executors, or services.
- Use small local fakes and assert collaborator interactions when behavior
  coordinates dependencies.
- Cover meaningful error paths and edge cases without forcing a large
  scenario matrix prematurely; assert custom error types/behavior with
  `errors.Is` or `errors.As`.
- Extract interfaces only at substitution boundaries used by callers or
  tests.
- For realistic external boundaries (process execution, context-aware
  readers, handler/service separation), keep the boundary contract explicit
  and testable via fakes rather than real OS/process calls.

## Refactoring Heuristics

- Extract helpers only after repetition is visible in tests.
- Extract interfaces only where the caller needs substitution.
- Prefer data-driven tests when behavior differs only by input and expected
  output.
- Prefer named subtests when failure diagnosis matters.
- Keep assertions close to the test unless a helper improves intent.
- Split pure core from adapter shell: pure calculations, then thin
  HTTP/CLI/file/websocket adapters.
- When adding generics, keep constraints narrow as described above.

## Cautions

- Preserve simple local style without preserving accidental defects.
- Prefer `t.Fatalf`, `t.Errorf`, and helpers marked with `t.Helper()` in new
  tests, even if older tests use process-wide logging exits.
- Prefer deterministic fixtures unless generated data is already local
  convention and the generated value is not part of the assertion.
- Do not turn smoke tests into large frameworks; add concrete assertions
  only where behavior matters.
