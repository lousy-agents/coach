---
name: mutation-hunter
description: Uncover test coverage gaps by applying semantic mutations to production TypeScript, Go, or Python code and identifying which mutations survive (tests still pass). Use when hunting mutants, validating test coverage, or finding behavioral gaps; surviving mutations indicate areas where tests are insufficient.
argument-hint: "<mutations> — number of mutations to hunt (e.g., 10). Optionally scope to files with --target <glob>. Use --lang ts|go|python to override deterministic language detection; defaults are language-aware."
allowed-tools: "read_file, edit_file, run_in_terminal, list_directory_contents, create_file"
---

# Mutation Hunter

You are a mutation testing agent. Your job is to find **surviving mutations** — semantic changes to production code that do not cause any tests to fail. Each surviving mutation is evidence of a test coverage gap.

## When to Use

Use this skill when a user asks to run mutation testing, hunt surviving mutants, validate behavioral test coverage, or find tests that would miss a regression in TypeScript, Go, or Python production code. Do not replace the agent-owned apply → test → classify → always-revert loop with an external mutation tool.

## Inputs

| Argument | Required | Description |
|:---|:---|:---|
| `mutations` | Yes | Number of mutations to attempt (e.g., `10`) |
| `--target` | No | Glob pattern for source files to target. The default is language-aware: TypeScript keeps `src/**/*.ts` excluding `*.d.ts`, `*.test.ts`, and `index.ts`; Go considers all non-test `*.go`; Python considers all eligible `*.py`. |
| `--lang` | No | Explicit language override: `ts`, `go`, or `python`. If omitted, use the deterministic selection rules below. |

## Language Selection

Select one language once before Step 1 and use that adapter for the entire run. The first matching rule wins, in this exact order:

1. An explicit `--lang ts`, `--lang go`, or `--lang python` flag.
2. The presence of `go.mod` selects `go`.
3. The presence of `package.json` plus either `tsconfig.json` or any `.ts`/`.tsx` source selects `ts`.
4. The presence of `pyproject.toml`, `setup.py`, `setup.cfg`, or any `requirements*.txt` selects `python`.
5. Otherwise, count supported source-file extensions under the target directory and select the majority: `.ts`/`.tsx` → `ts`, `.go` → `go`, `.py` → `python`.

An explicit unsupported `--lang` value is an input error. Conflicting project markers are not guessed at: precedence above resolves them deterministically, and the selected language is recorded in `metadata.language`. If extension counts tie, use the fixed order `ts`, then `go`, then `python`; if no supported source extension exists, stop with a clear language-detection error. A marker-based selection always takes precedence over extension counts.

## Language Adapters

The adapter is a small internal concept in this skill, not a runtime plugin system or external dependency. It supplies the baseline/test command, discovery and exclusions, priority paths, catalogue additions, and language-specific compile or syntax error classification.

### TypeScript adapter (existing behavior)

- Baseline/test command: `nvm use && npm test` (or just `npm test` if `nvm` is unavailable).
- Discovery: `find src -name "*.ts" ! -name "*.d.ts" ! -name "*.test.ts" ! -name "index.ts" | sort`.
- Priority folders: `src/entities/`, `src/use-cases/`, `src/gateways/`, and `src/lib/`.
- Exclusions: `*.d.ts`, `*.test.ts`, `index.ts`, and pure type-definition files. The composition root `src/index.ts` is excluded.
- TypeScript compile errors, including `tsc` failures reported by the test command, count as killed.

### Go adapter

- Baseline/test command: `go test ./...`. Add `-count=1` only when the agent observes test caching affecting classification.
- Discovery: all `*.go` files except `*_test.go` files, for example `find . -name "*.go" ! -name "*_test.go" | sort`.
- Priority paths, when present: `internal/`, `pkg/`, `domain/`, `service/`, and `usecase/`. Prefer those packages over `cmd/`; skip `cmd/main.go` when higher-value packages exist.
- In addition to shared mutations, target `err != nil` ↔ `err == nil` (or carefully remove an early `if err != nil { return ..., err }` guard), `==` ↔ `!=`, `true` ↔ `false`, and `break` ↔ `continue`.
- Return-value zero values include `nil`, `0`, `""`, `false`, and nil slices or maps.
- `go build` or `go test` compile failures count as killed.

### Python adapter

- Prefer `pytest`, or `python -m pytest`; if pytest is absent, use `python -m unittest`. Select the available command before the baseline and use that same command for every mutation.
- Discovery: all `*.py` files except `test_*.py`, `*_test.py`, files under `tests/`, `__pycache__/`, or `migrations/`, for example:

  ```bash
  find . -name "*.py" ! -name "test_*.py" ! -name "*_test.py" ! -path "*/tests/*" ! -path "*/__pycache__/*" ! -path "*/migrations/*" | sort
  ```

- Priority paths, when present: the package root and `src/`.
- In addition to shared mutations, target `is` ↔ `is not`, `in` ↔ `not in`, `True` ↔ `False`, `None` checks or early-return removal on `if x is None`, `and` ↔ `or`, and `break` ↔ `continue`.
- Return-value zero values include `None`, `0`, `""`, `False`, `[]`, and `{}`.
- `SyntaxError` or `ImportError` during the selected test command counts as killed.

## Workflow

### Step 1 — Pre-flight baseline

Ensure all tests pass before starting. If the baseline fails, abort and report the failure.

Run the selected adapter's baseline command:

```bash
# TypeScript: nvm use && npm test (or npm test when nvm is unavailable)
# Go:         go test ./...
# Python:     pytest, python -m pytest, or python -m unittest when pytest is absent
```

> If tests fail, output:
> ```json
> { "error": "Baseline test run failed. Fix failing tests before running mutation-hunter.", "details": "<test output>" }
> ```
> Then stop. Do not proceed with mutations on a broken baseline.

### Step 2 — Discover mutation targets

List production source files with the selected adapter's discovery rules. For TypeScript, retain the existing discovery command and focus folders:

```bash
find src -name "*.ts" ! -name "*.d.ts" ! -name "*.test.ts" ! -name "index.ts" | sort
```

For Go, list all `*.go` files except `*_test.go`; for Python, list eligible `*.py` files while excluding `test_*.py`, `*_test.py`, `tests/`, `__pycache__/`, and `migrations/`. Apply the adapter's priority paths. Skip files that are purely type definitions or otherwise have no executable code.

### Step 3 — Select mutation candidates

For each candidate file, read it and identify mutatable constructs from the shared catalogue and the selected language section. Build an internal list of `(file, line, mutation-type, original, mutated)` tuples. Select from this list randomly until you have reached the requested `mutations` count, favouring files with more complex logic and the adapter's priority paths.

### Step 4 — Hunt loop

For each mutation in your selection:

1. **Record** the original source of the target line.
2. **Apply** the mutation by editing the file (make the smallest possible change to a single construct).
3. **Run tests** with the selected adapter's test command:
   ```bash
   # TypeScript: npm test 2>&1
   # Go:         go test ./... 2>&1 (add -count=1 only for observed caching)
   # Python:     the selected pytest or unittest command 2>&1
   ```
4. **Classify** the result:
   - Tests **fail** — the mutation was **killed** ✅ (tests caught the change).
   - Tests **pass** — the mutation **survived** ❌ (test gap found).
5. **Revert** the mutation immediately by restoring the original line — never leave the code in a mutated state.
6. Log the result internally and continue.

> **Important:** Always revert before moving to the next mutation, even if the test runner crashes or times out. The codebase must be identical to the baseline when you finish. A failed revert stops the run and reports the affected file.

### Step 5 — Produce output

Write the final JSON report to stdout. Format is described in the **Output Format** section below.

## Mutation Catalogue

Apply **one mutation at a time** — never combine multiple changes in a single trial. Each mutation must be semantically meaningful (changes program behavior) rather than purely syntactic. The shared catalogue defines the cross-language intent; the TypeScript section below preserves the existing ten concrete TypeScript mutation types and examples exactly in behavior.

### Shared catalogue (all languages)

1. **Comparison / relational boundary and negation** — mutate `>`, `>=`, `<`, `<=`, equality, or inequality while preserving the language's syntax.
2. **Logical operators** — mutate `&&`/`||` in TypeScript and Go, or `and`/`or` in Python.
3. **Boolean literal flip** — mutate a boolean literal used as a value.
4. **Arithmetic operators** — mutate `+`/`-` or `*`/`/` where operands are numeric and the result is meaningful.
5. **Return value** — replace a result with the language-appropriate zero, empty, nil, `None`, or false value.
6. **Early-return / null-guard / error-guard removal** — remove a defensive guard when it protects an invalid state; do not remove an error-throwing guard indiscriminately.
7. **Off-by-one** — shift an index or slice/length boundary by ±1.
8. **Conditional inversion** — negate the complete condition while preserving language syntax.

### TypeScript-only compatibility catalogue (10 existing types)

#### 1. Comparison Operator Mutations

Change relational operators to probe boundary conditions:

| Original | Mutated | Rationale |
|:---|:---|:---|
| `> n` | `>= n` | Weakens strict lower bound |
| `< n` | `<= n` | Weakens strict upper bound |
| `>= n` | `> n` | Strengthens lower bound (off-by-one) |
| `<= n` | `< n` | Strengthens upper bound (off-by-one) |
| `=== x` | `!== x` | Inverts equality check |
| `!== x` | `=== x` | Inverts inequality check |

**Example:**
```typescript
// Original
if (size > MAX_SIZE) { throw new Error("Too large"); }

// Mutated
if (size >= MAX_SIZE) { throw new Error("Too large"); }
```

#### 2. Logical Operator Mutations

Replace logical connectives to expose missing compound-condition tests:

| Original | Mutated |
|:---|:---|
| `&&` | `\|\|` |
| `\|\|` | `&&` |

**Example:**
```typescript
// Original
if (name && name.length > 0) { ... }

// Mutated
if (name || name.length > 0) { ... }
```

#### 3. Boolean Literal Mutations

Negate boolean constants:

| Original | Mutated |
|:---|:---|
| `true` | `false` |
| `false` | `true` |

Only apply to boolean literals that are **used as values** (not as flags in control flow already covered by other mutation types).

#### 4. Arithmetic Operator Mutations

Swap arithmetic operators to expose miscalculation tests:

| Original | Mutated |
|:---|:---|
| `a + b` | `a - b` |
| `a - b` | `a + b` |
| `a * b` | `a / b` |
| `a / b` | `a * b` |

Only apply where both operands are numeric and the expression result is used meaningfully (not inside a template literal for display only).

#### 5. Return Value Mutations

Replace a function's return value with a type-compatible empty/zero value:

| Return type | Original | Mutated |
|:---|:---|:---|
| `string` | `return computedString` | `return ""` |
| `number` | `return computedNumber` | `return 0` |
| `boolean` | `return expr` | `return false` |
| `array` | `return computedArray` | `return []` |
| `object` | `return computedObject` | `return {} as typeof computedObject` |

**Example:**
```typescript
// Original
return statements.sort();

// Mutated
return [];
```

#### 6. Null-Guard / Early-Return Removal

Remove a defensive early-return to see whether callers handle `undefined`/`null` responses:

**Example:**
```typescript
// Original
if (!input) { return undefined; }

// Mutated — remove the guard entirely (or return without the check)
```

Only apply when the early-return protects against an invalid state. Do not apply to error-throwing guards (those are tested differently).

#### 7. Off-by-One Index Mutations

Shift array/string indices by ±1:

| Original | Mutated |
|:---|:---|
| `arr[i]` | `arr[i + 1]` |
| `arr[i]` | `arr[i - 1]` |
| `.slice(0, n)` | `.slice(0, n - 1)` |
| `.slice(0, n)` | `.slice(0, n + 1)` |

#### 8. Nullish / Optional-Chaining Mutations

Remove nullish coalescing or optional chaining:

| Original | Mutated |
|:---|:---|
| `value ?? defaultValue` | `value` (removes fallback) |
| `obj?.prop` | `obj.prop` (removes guard) |

#### 9. Object Property Mutations

Swap or omit an object property in a literal or spread to expose missing property assertions:

**Example:**
```typescript
// Original
return { name: input.name, version: input.version };

// Mutated
return { name: input.name, version: "" };
```

#### 10. Conditional Inversion

Negate the entire condition of an `if` statement:

**Example:**
```typescript
// Original
if (isValid(x)) { process(x); }

// Mutated
if (!isValid(x)) { process(x); }
```

### Go-only mutations

In addition to the shared catalogue, include Go constructs when present:

| Original | Mutated |
|:---|:---|
| `err != nil` | `err == nil` |
| `err == nil` | `err != nil` |
| `==` | `!=` |
| `!=` | `==` |
| `true` | `false` |
| `false` | `true` |
| `break` | `continue` |
| `continue` | `break` |

An error-guard mutation must remain a single semantic change. For example, mutate `if err != nil { return result, err }` to the opposite guard or remove that early return only when the resulting code remains a meaningful candidate. Return mutations may use `nil`, `0`, `""`, `false`, or a nil slice/map compatible with the function's result.

### Python-only mutations

In addition to the shared catalogue, include Python constructs when present:

| Original | Mutated |
|:---|:---|
| `is` | `is not` |
| `is not` | `is` |
| `in` | `not in` |
| `not in` | `in` |
| `True` | `False` |
| `False` | `True` |
| `and` | `or` |
| `or` | `and` |
| `break` | `continue` |
| `continue` | `break` |

For `if x is None` guards, remove or invert only the defensive early-return as one semantic mutation. Return mutations may use `None`, `0`, `""`, `False`, `[]`, or `{}` appropriate to the function's result.

## Output Format

Produce a single JSON object with the following schema. Write it to stdout.

```json
{
    "metadata": {
        "target": "src/",
        "language": "ts",
        "mutations_requested": 10,
        "timestamp": "<ISO-8601>"
    },
    "summary": {
        "files_analyzed": 5,
        "mutations_attempted": 10,
        "mutations_killed": 7,
        "mutations_survived": 3,
        "survival_rate": 0.3,
        "coverage_grade": "C"
    },
    "surviving_mutations": [
        {
            "id": "mut-001",
            "file": "src/use-cases/build-permission-policy.ts",
            "line": 42,
            "mutation_type": "comparison_operator",
            "original_code": "if (size > MAX_SIZE) {",
            "mutated_code": "if (size >= MAX_SIZE) {",
            "description": "Boundary condition weakened: `>` changed to `>=`",
            "coverage_gap": "No test exercises the exact boundary where size equals MAX_SIZE.",
            "advice": "Add a test case that produces a policy with size exactly equal to MAX_SIZE and assert that the function does NOT throw. Then add a second test at MAX_SIZE + 1 and assert that it DOES throw. This will pin down the inclusive/exclusive boundary."
        }
    ],
    "killed_mutations": [
        {
            "id": "mut-002",
            "file": "src/entities/policy-document.ts",
            "line": 10,
            "mutation_type": "boolean_literal",
            "original_code": "Effect: \"Allow\"",
            "mutated_code": "Effect: \"Deny\"",
            "description": "Effect field changed from Allow to Deny",
            "killed_by_test": "src/use-cases/build-permission-policy.test.ts"
        }
    ]
}
```

### Coverage Grade

Derive `coverage_grade` from `survival_rate` (surviving / attempted):

| Survival rate | Grade | Interpretation |
|:---|:---|:---|
| 0% | A | Excellent — tests killed every mutation |
| 1–10% | B | Good — minor gaps |
| 11–25% | C | Acceptable — some gaps worth addressing |
| 26–50% | D | Weak — significant test coverage gaps |
| > 50% | F | Poor — tests are insufficient to catch most regressions |

The `metadata.language` value must be exactly `ts`, `go`, or `python`. Keep every other report field and array shape unchanged.

## Advice Generation Guidelines

For each surviving mutation, generate `advice` that is:

1. **Specific** — reference the exact line and condition that survived, not generic advice like "add more tests".
2. **Actionable** — describe the exact input value or scenario that would kill the mutation (a test with `x === boundary` is better than "test the boundary").
3. **Contextual** — if the surviving mutation is in a validation function, the advice should mention testing the invalid input that should have been rejected.
4. **Minimal** — suggest the fewest tests needed to kill the mutation, not an exhaustive suite.

## Error Handling

| Situation | Action |
|:---|:---|
| Unsupported explicit language or no detectable language | Abort immediately with a clear input error; do not mutate |
| Baseline tests fail | Abort immediately, output error JSON, do not mutate |
| TypeScript compile error | Count as "killed" (compile error = detectable failure), revert, continue |
| Go `go build` / `go test` compile error | Count as "killed", revert, continue |
| Python `SyntaxError` / `ImportError` | Count as "killed", revert, continue |
| Test runner hangs > 60s | Kill the process, count as "killed" (timeout = detectable failure), revert, continue |
| File cannot be edited | Skip the mutation, log a warning in metadata |
| Revert fails | **Stop immediately**, report the partially-mutated file as an error so the user can restore it manually |

External mutation tools are not required. They may be used only for optional candidate discovery; the agent owns mutation application, one-at-a-time classification, reversion, and advice generation.

## Constraints

- **Never** leave the codebase in a mutated state when finished.
- **Never** apply more than one mutation at a time.
- Work in small, atomic changes — single-line edits preferred.
- Prefer mutations in business logic and the adapter's priority paths over composition roots, commands, or gateways.
- Never mutate test files, fixtures, generated files, migrations, pure type-definition files, or other adapter exclusions.
- TypeScript: never mutate `*.test.ts`, `*.d.ts`, `src/index.ts`, or pure type-definition files.
- Go: never mutate `*_test.go`; prefer `internal/`, `pkg/`, `domain/`, `service/`, or `usecase/`, and skip `cmd/main.go` when better packages exist.
- Python: never mutate `test_*.py`, `*_test.py`, files under `tests/`, `__pycache__/`, or `migrations/`.
- Keep TypeScript defaults, discovery, priority folders, exclusions, ten mutation types, examples, and test behavior compatible with the existing TypeScript path.
