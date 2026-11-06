---
name: go-testable-design
description: "Use when writing, adding, or improving Go tests, developing or refactoring Go code using TDD, test-first, red-green-refactor, outside-in or acceptance-first starts, executable documentation, behavior-focused assertions, or reviewing Go code for testability or package purity. Guides unit tests, table tests, subtests, helpers with t.Helper(), constructor injection for dependencies, CLI/process/filesystem boundaries, business logic, httptest, io/fs boundaries, context cancellation, goroutine and concurrency tests (channels, sync.WaitGroup, race detector), property tests, standard-library-first unit design, dual-track unit vs acceptance form, and acceptance suites that follow the repository's existing test harness."
argument-hint: "Optional: package, file path, bug, feature, or testing topic to work on"
allowed-tools: Read, Write, Edit, Bash, Grep, Glob
---

# Go Testable Design

Guide Go development with tests: small behavior first, executable documentation, clear boundaries, diagnostic assertions, and incremental refactoring. Informed by patterns from [`learn-go-with-tests`](https://github.com/quii/learn-go-with-tests), a community-maintained guide to TDD in Go.

## When to Use

Use this skill when the user asks to:

- Build or change Go code using TDD, tests first, red-green-refactor, or outside-in / acceptance-first starts.
- Add, improve, or explain Go unit or acceptance tests.
- Match acceptance / public-boundary coverage to the repository's existing test harness instead of inventing a new form.
- Design Go code around interfaces, `io.Reader`/`io.Writer`, `fs.FS`, `http.Handler`, `context.Context`, goroutines, channels, storage boundaries, or package purity.
- Refactor Go code while preserving behavior.
- Review Go code for testability, missing test cases, or IO creeping into decision packages.
- Learn or demonstrate Go concepts through tests.

Do not use this skill for non-Go projects, generic CI setup, or broad architecture work where tests are not part of the task.

## Core Rules

- For unit tests, prefer the standard library unless the repository already uses a focused dependency. Acceptance suites instead follow the repository's existing acceptance form (see [Unit vs Acceptance Tests](#unit-vs-acceptance-tests)).
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

### Unit vs Acceptance Tests

These are two different products and they do not share a default form.

- **Unit tests** cover a function, method, type, or package-internal seam. Standard-library `testing` is the default here, unless the repository already uses a focused dependency.
- **Acceptance tests** (also called public-boundary, feature, or spec suites) prove a feature or bug fix works through the program's public boundary. Their form is dictated by the repository, not by this skill: it may be stdlib `testing`, a BDD spec runner, a testify-style suite, a `godog` feature suite, or an in-tree custom harness.
- Classify a test by what it proves, not by its name or by whether its target is exported. A unit test proves one function, method, type, or handler in isolation, with its collaborators faked — a table test of an exported function and an `httptest` test of one handler are both unit tests. An acceptance test proves a feature or fix the way that criterion's user reaches it: through the command or endpoint with production wiring (a `Run(args…)` entry point, the assembled server), or, when the user is a library's consumer, through that consumer's scenario on the exported API. Production wiring is the module's real wiring, with external systems such as databases, networks, and processes replaced at their ports. A consumer's scenario drives one criterion through exported calls only, faking only what the consumer itself supplies, such as a rate source passed to a constructor. Find the boundary per criterion, since one repository can ship both a command and an importable package. A test of a type the criterion's user never calls directly, such as one in an `internal/` package, is a unit test, unless that type is the command's or server's entry point.
- Determine the acceptance form during **Orient**, before choosing a test shape. If no acceptance convention and no project policy exist, stdlib `testing` is the acceptance form too — write it the way the rest of the repository writes tests.
- NEVER invent a stdlib acceptance test when the repository mandates a different acceptance form. Conversely, NEVER import a spec runner into a repository that does not already use one: adding a runner is a dependency decision owned by the module's maintainers, not a test decision.
- See [`references/go-test-patterns.md`](./references/go-test-patterns.md) for the detection commands and a worked example of the same criterion in two acceptance forms.

### Acceptance-Criteria-First Structure (Given/When/Then)

- The principle is that **the test's structure should read as the criterion** — not that any particular helper style is required. Capture acceptance criteria in structure and naming instead of explaining them only in a comment (e.g. an `// AC-1.8: ...` annotation). The test should demonstrate the criterion; the comment should not be the only place it is recorded.
- The vehicle is whatever the repository already uses: a spec runner's `Describe`/`When`/`It` blocks, stdlib `t.Run` subtests, named `given`/`when`/`then` helpers, or another in-tree runner's block structure. Do not switch vehicles to satisfy this principle.
- Suggested shape when the vehicle is stdlib helpers: `given...` builds the starting state or fixture, `when...` performs the action under test, and `then...`/`assert...` checks the observable outcome. Keep each helper small and focused on one concern; for helpers that accept `*testing.T`/`testing.TB` (typically `then`/`assert` helpers), call `t.Helper()` so failures point at the caller — pure `given`/`when` builders that don't take `t` don't need it.
- Subtest names should read as the behavioral rule itself (`"rejects overdraft withdrawals"`, `"cancels in-flight work when the context ends"`) so the test file reads like a spec without needing the comment to translate intent.
- It is fine to keep a short traceability comment (e.g. referencing an acceptance-criteria ID from a spec) above a test, but it must not be the only description of the behavior — the extracted `given`/`when`/`then` structure should independently make the criterion legible.
- Do not over-extract: a single straight-line test with clear variable names can already satisfy this if it reads like the criterion. Reach for `given`/`when`/`then` helpers when a comment is currently doing the work that structure and naming should do instead.
- See [`references/go-test-patterns.md`](./references/go-test-patterns.md) for a before/after example converting an AC comment into Given/When/Then helpers.

### Package Fitness (Dependency Rule)

A package's import list decides how testable it is before any test exists: a package that imports no IO needs no fakes, which is what makes the constructor-injection and boundary-interface rules above achievable.

- Keep the packages that hold calculations and decisions free of framework, network, database, and filesystem imports. Put those adapters in packages at the edge and wire them together in the composition root.
- Declare interfaces at the use site: the consuming package defines the small interface it needs, and the producing package returns concrete types.
- Verify purity instead of asserting it. The first command reports IO reached through your module's own packages; the second lists third-party dependencies for review. Keep them separate — a single `go list -deps` grep inherits stdlib-internal imports and reports `os` for any package that merely calls `fmt.Errorf`. Both filter on `go list`'s own module metadata rather than string-matching the module path, so neither misreports under `go.work`. Each command is a single `go list` (no `xargs`) whose output is captured before it is filtered: a bad path or undownloaded deps makes `go list` exit non-zero with empty stdout, and piping that straight into `grep` exits 1 exactly as a pure package does (`set -o pipefail` does not separate them either). Each check exits 0 when the package is clean, 1 when it found something to report, and 2 when the check did not run — never read 2 as a pass. For the third-party check, 1 means the list needs review, not that the package violates anything. The IO check excludes `net/url` and `net/netip`, which parse values and do no IO:

  ```bash
  # stdlib IO reached through your own packages
  ( imports=$(go list -deps -f '{{if and .Module .Module.Main}}{{range .Imports}}{{.}}{{"\n"}}{{end}}{{end}}' ./internal/pricing) ||
      { echo "IO check did not run" >&2; exit 2; }
    printf '%s\n' "$imports" | sort -u | grep -E '^(os|net|log|database/sql)(/|$)' | grep -vxE 'net/(url|netip)' && exit 1
    echo "no IO imports" )
  # third-party dependencies
  ( deps=$(go list -deps -f '{{if and .Module (not .Module.Main)}}{{.ImportPath}}{{end}}' ./internal/pricing) ||
      { echo "third-party check did not run" >&2; exit 2; }
    [ -z "$deps" ] && { echo "no third-party dependencies"; exit 0; }
    printf '%s\n' "$deps"; exit 1 )
  ```

- Read the third-party list, do not just count it. Both checks are transitive, so a dependency reached only through an internal sub-package that exists to own it is the intended shape, not a violation — what you are looking for is framework and IO creep into the packages that hold decisions.

- See [`references/go-test-patterns.md`](./references/go-test-patterns.md) for a worked purity check on a pure and an impure package.

## Mandatory Test Quality Bar

Before finalizing any Go test, check it against these requirements:

- **Behavior contract:** The test name or subtest name describes a user-visible rule, protocol, state transition, error condition, or boundary contract.
- **Executable documentation:** The arrange/act/assert flow shows the meaningful example. Expected values are visible at the call site unless a helper makes the domain intent clearer.
- **Refactor tolerance:** A production refactor that preserves the public behavior should not break the test. If it would, the test is probably cementing implementation.
- **Diagnostic failure:** Each assertion failure identifies what behavior was expected, the important input or state, and the observed value. Avoid failures that only say `expected true`, `not equal`, or `wrong result`.
- **Legitimate interaction checks:** Spy/mock assertions are reserved for observable boundary contracts, such as command arguments, repository writes, emitted events, cancellation calls, or external requests. Avoid verifying incidental call order or helper calls.
- **No duplicate algorithms:** Do not compute `want` by reimplementing the production algorithm in the test. Use concrete examples, fixtures, properties, or independent invariants.
- **Structure over comments for acceptance criteria:** When a test exists to satisfy a specific acceptance criterion, express it through the structure the repository's runner provides — `given`/`when`/`then` helpers, a descriptive subtest name, or the runner's own nested blocks — rather than relying on a comment to explain the mapping.

## Procedure

1. **Orient**
   - Inspect `go.mod`, package layout, existing tests, and naming conventions.
   - **Detect the repository's acceptance-test convention before choosing any test shape.** Check every signal: existing acceptance suite files (`*_acceptance_test.go` or equivalently named suites, and `*_test.go` under `acceptance/` or `e2e/` — not `features/`, `spec/`, or `specs/`, which are common package names); suite/spec-runner packages imported by the repository's `*_test.go` files (e.g. `testify/suite`, `ginkgo` — not unit-assertion helpers like `testify/assert` or `gomega`; read imports, not `go.mod`, which lists modules rather than packages and whose `// indirect` lines are not this repository's choice); a suite entrypoint (a `RunSpecs`-style bootstrap, a `suite.Run` call, or a `godog.TestSuite` or `godog.RunWithOptions` call, or, when the tests import godog, the `.feature` files a godog suite runs — a `TestMain` counts only through the runner call inside it, never for setup, teardown, or flag parsing); acceptance targets in `Makefile`, `mise.toml`, `Taskfile.yml`, or CI workflows; and stated policy in project agent docs (`AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`). Ignore `vendor/` and `testdata/` for every signal. A runner import counts only with a matching suite file or entrypoint.
   - Record the answer explicitly: which acceptance form the repository mandates, or that it has none. **No signals means stdlib `testing`** — do not add a runner. Unit-test assertion helpers alone are not an acceptance convention. See [`references/go-test-patterns.md`](./references/go-test-patterns.md) for the detection commands and how to read conflicting signals.
   - Identify the smallest package or file that owns the behavior.
   - If `go` is available, run the narrowest baseline test first:

     ```bash
     go test ./path/to/package
     ```

2. **Choose the Test Shape**
   - **New feature or bug fix that changes externally visible behavior: start outside-in.** The first failing test exercises the boundary where that criterion's user reaches it — a CLI invocation or HTTP endpoint, or the exported package API when the user is a library's consumer — rather than an internal helper.
   - If this is acceptance coverage for a feature or bug fix, use the repository's acceptance form identified in Orient; the shapes below are for unit and other non-acceptance tests.
   - Inside-out is legitimate for a pure internal helper, an algorithmic core, or a well-understood domain whose public API is already settled: start at the unit and skip the boundary test.
   - Pure functions: use direct assertions, then table tests once cases multiply.
   - Methods with mutation: assert state before and after, and cover error paths.
   - Business logic with collaborators: inject dependencies through constructors and test with small local fakes.
   - HTTP handlers: use `net/http/httptest`; assert status, headers, body, and collaborator calls.
   - CLI/process/filesystem code: inject readers, writers, env lookup, filesystem access, and command execution.
   - File parsing or rendering: prefer `fs.FS`, `strings.Reader`, `bytes.Buffer`, temp files, and approval-style fixtures when useful.
   - Goroutines/concurrency: benchmark before optimizing, run with `go test -race`, coordinate results with channels or `sync.WaitGroup` instead of shared mutable state, and add `select`/`context.Context` timeouts so tests fail fast instead of hanging.
   - Properties or reversible transformations: add `testing/quick` after concrete examples establish the expected behavior.

3. **Write the First Failing Test**
   - Working outside-in, write the call you wish existed and invent the ports, spies, and fakes it needs from inside the test: the test is the API's first consumer, so an awkward test is evidence of an awkward API — change the signature, not the test. See [`references/go-test-patterns.md`](./references/go-test-patterns.md) for a worked walkthrough.
   - In stdlib tests, name the behavior with `t.Run` when multiple cases are expected; in an acceptance suite, use the block structure that suite already uses.
   - Phrase test and subtest names as contract statements, such as `rejects overdraft withdrawals`, `writes JSON with a 201 status`, or `cancels in-flight work when the context ends`.
   - Put expected values in the test, not hidden inside helpers.
   - Write assertion messages that include the protected behavior, relevant inputs, and `got`/`want`.
   - Introduce helpers only after the test starts repeating setup or assertion detail.
   - Run the new test and watch it fail before changing production code. A test that passes before the change does not yet exercise the new behavior.

4. **Implement Simply**
   - Hard-code when that is the honest smallest step.
   - Parameterize once the second test forces it.
   - Working outside-in, once the boundary test is green return to step 3 for the pieces that carry real logic: their unit tests own the edge cases the boundary test should not enumerate.
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
   - Before finalizing, sweep the tests you touched for a comment that is the *only* record of a behavior — an `// AC-1.8: ...` annotation, a prose rule above a test function, a note explaining what the assertions add up to. Convert each one into structure: a named subtest, `given`/`when`/`then` helpers, or the runner's own block name. The traceability ID may stay; the rule it was carrying moves into the code. The grep below is a candidate finder, not a convert-everything mandate: skip hits where structure and naming already carry the rule (ordinary `// should not panic` notes, leftover prose next to a descriptive `t.Run`, etc.).

     ```bash
     grep -rnE --include='*_test.go' '^[[:space:]]*//.*([Aa][Cc]-[0-9]|[Rr]equirement|[Cc]riteri|\bmust\b|\bshould\b)' path/to/package
     ```

## Test Patterns

Use idiomatic Go test structure:

```go
func TestPrice(t *testing.T) {
    t.Run("takes 4% off the subtotal for members", func(t *testing.T) {
        got := Price(Order{Subtotal: 100, Member: true})
        want := Money(96)

        if got != want {
            t.Errorf("4%% member discount on subtotal 100: got %v, want %v", got, want)
        }
    })
}
```

Use helpers when they make intent clearer:

```go
func assertEqual[T comparable](t testing.TB, what string, got, want T) {
    t.Helper()

    if got != want {
        t.Errorf("%s: got %v, want %v", what, got, want)
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
- Checking that a package meant to stay pure has not picked up IO, network, or third-party dependencies.
- Detecting whether the repository mandates a particular acceptance-test form before writing acceptance coverage.
- Starting a feature or bug fix outside-in: a failing boundary test that invents the ports it needs, then drilling inward with unit tests.

## Output Expectations

When reporting work:

- State which package/files changed.
- State which tests were added or changed, which of them are acceptance coverage and which are unit tests, and either the acceptance form used with the signal that decided it (or that the repository has none) or, if no acceptance test was written, why.
- State the verification command and result.
- If tests could not run, state why without implying they passed.
