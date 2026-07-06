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
