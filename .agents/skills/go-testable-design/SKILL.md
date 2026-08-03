---
name: go-testable-design
description: "Use when writing, adding, or improving Go tests, developing or refactoring Go code using TDD, test-first, red-green-refactor, executable documentation, behavior-focused assertions, or reviewing Go code for testability. Guides unit tests, table tests, subtests, helpers with t.Helper(), constructor injection for dependencies, CLI/process/filesystem boundaries, business logic, httptest, io/fs boundaries, context cancellation, goroutine and concurrency tests (channels, sync.WaitGroup, race detector), property tests, and standard-library-first design."
argument-hint: "Optional: package, file path, bug, feature, or testing topic to work on"
allowed-tools: Read, Write, Edit, Bash, Grep, Glob
---

# Go Testable Design

Guide Go development with tests: small behavior first, executable documentation, clear boundaries, diagnostic assertions, and incremental refactoring. Informed by patterns from [`learn-go-with-tests`](https://github.com/quii/learn-go-with-tests), a community-maintained guide to TDD in Go.

## When to Use

Use this skill when the user asks to:

- Build or change Go code using TDD, tests first, or red-green-refactor.
- Add, improve, or explain Go tests.
- Design Go code around interfaces, `io.Reader`/`io.Writer`, `fs.FS`, `http.Handler`, `context.Context`, goroutines, channels, or storage boundaries.
- Refactor Go code while preserving behavior.
- Review Go code for testability or missing test cases.
- Learn or demonstrate Go concepts through tests.

Do not use this skill for non-Go projects, generic CI setup, or broad architecture work where tests are not part of the task.

## Core Rules

- Prefer the standard library unless the repository already uses a focused dependency.
- Match nearby test style; keep new tests succinct, direct, and behavior-focused.
- Start from externally visible behavior: exported function, method, handler, CLI, file reader, or concurrent contract.
- Tests MUST read like executable documentation for the behavior being implemented: names, setup, inputs, and expectations should explain the contract without requiring the reader to inspect production internals.
- Assert observable outcomes, public errors, persisted effects, emitted output, or boundary interactions that are part of the contract. Do not assert private implementation steps merely because the current production code happens to use them.
- Failure messages MUST be diagnostic: include the behavior being protected plus relevant input/context and `got`/`want` values, so a human or agent can understand the intended production behavior from the failure alone.
- Write the smallest failing test that names the behavior.
- Make the smallest production change that passes.
- Refactor only after behavior is covered.
- Keep test helpers small; mark helpers that accept `*testing.T`/`testing.TB` with `t.Helper()`.
- Prefer constructor injection for dependencies that touch external state.
- Use interfaces at boundaries, not everywhere.
- Keep production APIs zero-value friendly where practical.
- Do not hide meaningful errors from tests; assert them.
- Avoid sleeps in tests unless the behavior is explicitly timing-based. Prefer fake clocks, channels, contexts, or retry helpers.

### Acceptance-Criteria-First Helpers (Given/When/Then)

- Prefer capturing acceptance criteria in the test's structure — named `given`/`when`/`then` helpers or descriptive `t.Run` subtests — instead of explaining them only in a comment (e.g. an `// AC-1.8: ...` annotation). The test should demonstrate the criterion; the comment should not be the only place it is recorded.
- Suggested shape: `given...` builds the starting state or fixture, `when...` performs the action under test, and `then...`/`assert...` checks the observable outcome. Keep each helper small and focused on one concern; for helpers that accept `*testing.T`/`testing.TB` (typically `then`/`assert` helpers), call `t.Helper()` so failures point at the caller — pure `given`/`when` builders that don't take `t` don't need it.
- Subtest names should read as the behavioral rule itself (`"rejects overdraft withdrawals"`, `"cancels in-flight work when the context ends"`) so the test file reads like a spec without needing the comment to translate intent.
- It is fine to keep a short traceability comment (e.g. referencing an acceptance-criteria ID from a spec) above a test, but it must not be the only description of the behavior — the extracted `given`/`when`/`then` structure should independently make the criterion legible.
- Do not over-extract: a single straight-line test with clear variable names can already satisfy this if it reads like the criterion. Reach for `given`/`when`/`then` helpers when a comment is currently doing the work that structure and naming should do instead.
- See [`references/go-test-patterns.md`](./references/go-test-patterns.md) for a before/after example converting an AC comment into Given/When/Then helpers.

## Mandatory Test Quality Bar

Before finalizing any Go test, check it against these requirements:

- **Behavior contract:** The test name or subtest name describes a user-visible rule, protocol, state transition, error condition, or boundary contract.
- **Executable documentation:** The arrange/act/assert flow shows the meaningful example. Expected values are visible at the call site unless a helper makes the domain intent clearer.
- **Refactor tolerance:** A production refactor that preserves the public behavior should not break the test. If it would, the test is probably cementing implementation.
- **Diagnostic failure:** Each assertion failure identifies what behavior was expected, the important input or state, and the observed value. Avoid failures that only say `expected true`, `not equal`, or `wrong result`.
- **Legitimate interaction checks:** Spy/mock assertions are reserved for observable boundary contracts, such as command arguments, repository writes, emitted events, cancellation calls, or external requests. Avoid verifying incidental call order or helper calls.
- **No duplicate algorithms:** Do not compute `want` by reimplementing the production algorithm in the test. Use concrete examples, fixtures, properties, or independent invariants.
- **Structure over comments for acceptance criteria:** When a test exists to satisfy a specific acceptance criterion, prefer expressing it through `given`/`when`/`then` helpers or a descriptive subtest name rather than relying on a comment to explain the mapping.

## Procedure

1. **Orient**
   - Inspect `go.mod`, package layout, existing tests, and naming conventions.
   - Identify the smallest package or file that owns the behavior.
   - If `go` is available, run the narrowest baseline test first:

     ```bash
     go test ./path/to/package
     ```

2. **Choose the Test Shape**
   - Pure functions: use direct assertions, then table tests once cases multiply.
   - Methods with mutation: assert state before and after, and cover error paths.
   - Business logic with collaborators: inject dependencies through constructors and test with small local fakes.
   - HTTP handlers: use `net/http/httptest`; assert status, headers, body, and collaborator calls.
   - CLI/process/filesystem code: inject readers, writers, env lookup, filesystem access, and command execution.
   - File parsing or rendering: prefer `fs.FS`, `strings.Reader`, `bytes.Buffer`, temp files, and approval-style fixtures when useful.
   - Goroutines/concurrency: benchmark before optimizing, run with `go test -race`, coordinate results with channels or `sync.WaitGroup` instead of shared mutable state, and add `select`/`context.Context` timeouts so tests fail fast instead of hanging.
   - Properties or reversible transformations: add `testing/quick` after concrete examples establish the expected behavior.

3. **Write the First Failing Test**
   - Name the behavior with `t.Run` when multiple cases are expected.
   - Phrase test and subtest names as contract statements, such as `rejects overdraft withdrawals`, `writes JSON with a 201 status`, or `cancels in-flight work when the context ends`.
   - Put expected values in the test, not hidden inside helpers.
   - Write assertion messages that include the protected behavior, relevant inputs, and `got`/`want`.
   - Introduce helpers only after the test starts repeating setup or assertion detail.

4. **Implement Simply**
   - Hard-code when that is the honest smallest step.
   - Parameterize once the second test forces it.
   - Extract functions, interfaces, or generic helpers only when tests show repeated structure.

5. **Refactor Under Tests**
   - Move side effects behind small interfaces or function fields.
   - Keep core calculations pure and adapters thin.
   - For application code, split composition roots (`cmd/.../main.go`) from reusable package logic.
   - After refactoring, scan tests for implementation coupling: private helper assertions, incidental call-order checks, exact intermediate values, or copied production logic.

6. **Verify**
   - Run the narrow package test after each meaningful change.
   - Run broader tests before finalizing when the change touches shared APIs:

     ```bash
     go test ./...
     ```

   - If the local environment lacks Go or dependencies, report that clearly and include the exact command that should be run.

## Test Patterns

Use idiomatic Go test structure:

```go
func TestPrice(t *testing.T) {
    t.Run("applies the member discount before tax", func(t *testing.T) {
        got := Price(Order{Subtotal: 100, Member: true})
        want := Money(96)

        if got != want {
            t.Errorf("member discount should be applied before tax for subtotal 100: got %v, want %v", got, want)
        }
    })
}
```

Use helpers when they make intent clearer:

```go
func assertEqual[T comparable](t testing.TB, got, want T) {
    t.Helper()

    if got != want {
        t.Errorf("got %v, want %v", got, want)
    }
}
```

Prefer domain-specific helpers when they improve failures:

```go
func assertBalance(t testing.TB, account Account, want Money) {
    t.Helper()

    if got := account.Balance(); got != want {
        t.Fatalf("account balance after transaction: got %v, want %v", got, want)
    }
}
```

Use spies and stubs at boundaries:

```go
type SpyStore struct {
    recorded []string
}

func (s *SpyStore) RecordWin(name string) {
    s.recorded = append(s.recorded, name)
}
```

## Test Pattern Reference

Read [`references/go-test-patterns.md`](./references/go-test-patterns.md) when the task involves:

- Choosing between test styles for a particular Go topic: pure functions, HTTP/JSON handlers, file/rendering/CLI boundaries, time-based orchestration, websockets, dependency injection/mocking, reflection/generics/property tests, or business logic with collaborators.
- Making tests read as executable documentation, writing diagnostic assertion failures, and avoiding implementation-coupled anti-patterns.
- Testing goroutines and concurrent code: benchmarking before optimizing, avoiding shared-state data races, using the race detector (`go test -race`), coordinating with channels or `sync.WaitGroup`, and adding `select`/timeout guards so tests fail fast.
- Existing Go projects with established handwritten test conventions.
- Constructor injection for testable business logic, handlers, CLIs, process execution, filesystem access, or other external boundaries.
- Preserving simple local test style while improving isolation and test hygiene.
- Converting acceptance-criteria comments into Given/When/Then-style helpers or subtests.

## Output Expectations

When reporting work:

- State which package/files changed.
- State which tests were added or changed.
- State the verification command and result.
- If tests could not run, state why without implying they passed.
