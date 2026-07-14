# coach

`coach` is an experimental AI coach designed to help software engineers and autonomous agents build better software. It parses source code syntactically and flags code smells, design issues, and structural metrics.

## Packages

- [`pkg/semantics`](./pkg/semantics) — Deterministic structural analysis of Go, TypeScript, and TSX source bytes (validates syntax, extracts imports, computes branching metrics, and detects constructor-like patterns).
- [`pkg/githubingest`](./pkg/githubingest) — Optional GitHub App-authenticated single-file reader via the GitHub Contents API.

---

## Installation

### Go Packages
To install the Go packages:

```sh
go get github.com/lousy-agents/coach/pkg/semantics

# Optional: for GitHub App content ingestion
go get github.com/lousy-agents/coach/pkg/githubingest
```

### JavaScript / TypeScript Bindings (`@lousy-agents/coach-semantics`)
The JS/TS bindings are currently packaged for Node.js (ESM-only). 

> [!NOTE]
> Because `coach` is in an active experimental phase, the npm package is not yet published to the public npm registry. To consume it, you must clone the repository and build the library locally:

> [!IMPORTANT]
> **Build Prerequisites:** Because the package compiles its underlying parser engine locally during installation, you must have:
> - **Node.js** (>= 20)
> - **Go** (>= 1.25.0)
> - **C Toolchain** (e.g., GCC or Clang): The Go parser engine compiles with CGO enabled by default. If a C toolchain is not available on your system, you can disable CGO by setting `CGO_ENABLED=0` (e.g., `CGO_ENABLED=0 npm install`) to build using the pure-Go engine fallback automatically.

1. **Clone and Build:**
   ```sh
   git clone https://github.com/lousy-agents/coach.git
   cd coach/js/semantics
   npm install   # Compiles the underlying parser engine and packages TS code
   ```

2. **Link or Reference the Package:**
   In your client application, add the local path as a dependency:
   ```sh
   cd ~/your-app
   npm install /path/to/coach/js/semantics
   ```

---

## `coach codesignal` CLI Preview

`cmd/coach` provides a `codesignal` subcommand: a local, deterministic preview of `pkg/codesignal` you can run directly against a Git checkout, without any GitHub App, worker, or model/LLM configuration.

Download the latest `coach` binary for macOS from the [GitHub Releases page](https://github.com/lousy-agents/coach/releases). Each tagged release publishes separate archives for Apple silicon (`darwin_arm64`) and x86-64 Intel Macs (`darwin_x86_64`), a `checksums.txt` file, and a cosign signature bundle.

```sh
ARCH=darwin_arm64  # or darwin_x86_64

curl -LO https://github.com/lousy-agents/coach/releases/latest/download/coach_${ARCH}.tar.gz
curl -LO https://github.com/lousy-agents/coach/releases/latest/download/checksums.txt
curl -LO https://github.com/lousy-agents/coach/releases/latest/download/checksums.txt.bundle

# Verify the checksums file was signed by this repository's release workflow
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-identity-regexp '^https://github.com/lousy-agents/coach/.github/workflows/release.yml@refs/tags/v.*$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt

# Verify the archive against the signed checksums
shasum -a 256 -c --ignore-missing checksums.txt

# Extract the binary
tar -xzf coach_${ARCH}.tar.gz
```

Move the extracted `coach` binary somewhere on your `PATH`.

> [!NOTE]
> From a local clone, you can still build/install it with:
> ```sh
> go install ./cmd/coach
> ```

**Prerequisites:** a local Git checkout and the `git` executable in `PATH`. Building from source also requires Go 1.25+ (matching `go.mod`).

### Usage

Run it from inside a Git worktree, pointing `--base` at the revision you want to diff against:

```sh
coach codesignal --base <ref>
```

- `--base` is required; it can be any ref Git can resolve to a commit (a branch, tag, or SHA).
- `--format` defaults to `text`; pass `--format=json` for machine-readable output.

**Text example** (`coach codesignal --base <ref>`), after a commit adds a function that mutates a caller-owned pointer:

```
files analyzed: 1, active signals: 1, diagnostics: 0
path: config.go
line: 12
lifecycle: introduced
changed: true
evidence: cfg.Timeout
why it matters: Mutating a caller-owned input can create behavior that is not visible from the function signature, make outcomes dependent on call ordering, introduce temporal coupling, make tests and local reasoning more difficult, and surprise callers that expect an input to remain unchanged.
recommendation: Return a copy instead of mutating the caller's value, or document/rename this function to make the in-place mutation explicit.
```

**JSON example** (`coach codesignal --base <ref> --format=json`), for the same change:

```json
{
  "schema_version": "1",
  "scope": { "revision": "3474e2c...", "base": "ece3690..." },
  "summary": {
    "files_analyzed": 1,
    "files_with_diagnostics": 0,
    "active_signals": 1,
    "introduced_signals": 1,
    "existing_signals": 0,
    "resolved_signals": 0
  },
  "signals": [
    {
      "id": "sig_88ec28c6...",
      "fingerprint": "fp_dcf2afc2...",
      "rule_id": "state.hidden_input_mutation",
      "rule_version": "1",
      "kind": "hidden_input_mutation",
      "category": "state_management",
      "severity": "medium",
      "confidence": "medium",
      "lifecycle": "introduced",
      "changed": true,
      "path": "config.go",
      "subject": "ApplyDefaults:cfg",
      "location": { "start_byte": 137, "end_byte": 148, "start_row": 11, "start_col": 1, "end_row": 11, "end_col": 12 },
      "evidence": "cfg.Timeout",
      "why_it_matters": "Mutating a caller-owned input can create behavior that is not visible from the function signature, make outcomes dependent on call ordering, introduce temporal coupling, make tests and local reasoning more difficult, and surprise callers that expect an input to remain unchanged.",
      "recommendation": "Return a copy instead of mutating the caller's value, or document/rename this function to make the in-place mutation explicit.",
      "suggested_skill": "refactor-hidden-mutation",
      "provenance": { "producer": "semantics", "finding_kind": "mutates_input" }
    }
  ]
}
```

### Exit status

- `0` — the CLI completed its analysis, regardless of whether any signals or diagnostics were reported. A quiet report with only diagnostics and zero signals is still a normal, exit-0 outcome.
- `1` — an operational failure: the working directory is not a Git worktree, `--base` cannot be resolved, `git` is not found in `PATH`, or an internal analysis step fails. One actionable message goes to stderr; nothing is written to stdout.
- `2` — a usage error: `--base` is missing, or `--format` is not `text` or `json`. Usage guidance goes to stderr; nothing is written to stdout.

### Scope and limitations

- **Advisory only.** It surfaces deterministic structural signals; it does not judge correctness or block anything on its own.
- **Go, TypeScript, and TSX only.** Changed files in other languages are skipped (with an `unsupported_language` diagnostic).
- **Does not execute code.** All analysis is static, over source bytes read via `git show`.
- **No runtime proof, no cross-file analysis.** It cannot prove a defect exists or trace causality across files; each file is analyzed independently.
- **Local-only, zero external configuration.** It never contacts GitHub, a model/LLM API, or any other network service — see `internal/codesignalcli/dependencies_test.go`'s `TestNoExternalDependencies` for the enforced boundary (no `net/http`, no GitHub client, anywhere in its dependency graph).
- **Renames and copies are not analyzed for lifecycle continuity.** A renamed or copied file produces an `unsupported_change_type` diagnostic instead of being diffed against its old path.

### Signal lifecycle and `changed`

Every signal carries a `lifecycle`, computed by comparing HEAD against the resolved merge-base:

- `introduced` — present at HEAD, not present at the merge-base.
- `existing` — present at both HEAD and the merge-base.
- `resolved` — present at the merge-base, not present at HEAD.
- `unknown` — the merge-base side of the file could not be analyzed (e.g. it had a syntax error), so lifecycle can't be determined.

`changed` is a separate boolean: it's `true` when the signal's HEAD location overlaps a line the diff marks as changed, independent of `lifecycle` — an untouched, pre-existing signal in a file that had other lines changed is `changed: false`.

### Locations: 0-based JSON vs 1-based text

JSON output reports `Location` fields (`start_row`, `start_col`, etc., see `pkg/semantics/result.go`) as 0-based, matching Tree-sitter's own convention. Text-mode output's `line:` field adds 1 to `start_row` for a human-friendly 1-based display line.

---

## `pkg/semantics` Quickstart

`pkg/semantics` operates purely on raw bytes, meaning you don't need a file system to analyze code.

```go
package main

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func main() {
	// Initialize the analyzer
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		panic(err)
	}

	sourceBytes := []byte(`
		package main
		func Hello() string {
			return "world"
		}
	`)

	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "hello.go",
		Language: semantics.LanguageGo,
		Content:  sourceBytes,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("Status:", result.ParseStatus) // "ok"
	for _, f := range result.Findings {
		fmt.Printf("Finding: %s (Location: %v)\n", f.Kind, f.Location)
	}
}
```

### Error Handling

If a file has syntax errors, `AnalyzeBytes` returns a partial `*Result` along with an error wrapped as `ErrSyntax`. You can extract detailed syntax issues using `errors.As`:

```go
import "errors"

// ...

result, err := analyzer.AnalyzeBytes(ctx, input)
if errors.Is(err, semantics.ErrSyntax) {
	var syntaxErr *semantics.SyntaxError
	if errors.As(err, &syntaxErr) {
		for _, issue := range syntaxErr.Issues {
			fmt.Printf("Syntax issue: %s at %v\n", issue.Kind, issue.Location)
		}
	}
}
```

Other sentinel errors:
- `ErrEmptyContent` — The provided input content is empty.
- `ErrUnsupportedLanguage` — The file extension or language is not supported.
- `ErrFileTooLarge` — The source file exceeds limits.
- `ErrBinaryContent` — The file appears to be binary.
- `ErrParseFailure` — General Tree-sitter parsing failure.

---

## Coaching Findings

Beyond parsing metrics, `coach` analyzes code constructs to flag specific design issues. The most common is `mutates_input`.

### `mutates_input` Finding
`mutates_input` flags functions or methods that mutate a parameter in place. This can lead to hard-to-debug "spooky action at a distance" bugs.

For example, this Go function mutates its input in place:
```go
func ApplyDefaults(cfg *Config) {
    cfg.Timeout = 30 * time.Second // Mutates the caller's Config
}
```

This generates a finding detailing the mutation:
```json
{
  "kind": "mutates_input",
  "name": "ApplyDefaults:cfg",
  "location": { "start_byte": 26, "end_byte": 36, "start_row": 1, "start_col": 4, "end_row": 1, "end_col": 14 },
  "confidence": "medium",
  "evidence": "cfg.Timeout",
  "recommendation": "Return a copy instead of mutating the caller's value, or document/rename this function to make the in-place mutation explicit.",
  "suggested_skill": "refactor-hidden-mutation"
}
```

A safer alternative returns the updated value:
```go
func WithDefaults(cfg Config) Config {
    cfg.Timeout = 30 * time.Second
    return cfg
}
```

> [!TIP]
> **Syntactic Constraint:** The `mutates_input` checker is purely syntactic and conservative. It does not perform interprocedural analysis or follow reference aliases across scopes.

### Other Supported Findings
- `constructor_func` (Go) — Flags factory or constructor functions.
- `pointer_return` (Go) — Flags functions returning a pointer to a struct.
- `tight_coupling` (TS/TSX) — Flags class/object patterns with high coupling.

---

## JavaScript / TypeScript Quickstart

Here is how you can perform syntactic analysis in a Node.js project:

```ts
import { readFile } from "node:fs/promises";
import { createAnalyzer, SemanticsSyntaxError } from "@lousy-agents/coach-semantics";

// Initialize the analyzer child process
const analyzer = await createAnalyzer();

try {
  const content = await readFile("widget.ts");
  const result = await analyzer.analyzeBytes({
    path: "widget.ts",
    language: "typescript", // "go", "typescript", or "tsx"
    content: content,
  });

  console.log("Status:", result.parse_status);
  console.log("Metrics:", result.metrics);
  console.log("Findings:", result.findings);
} catch (err) {
  if (err instanceof SemanticsSyntaxError) {
    // Access the partial Result containing syntax errors
    console.log("Syntax Errors:", err.partialResult.syntax_errors);
  } else {
    console.error("Analysis failed:", err);
    throw err;
  }
} finally {
  // Always clean up the analyzer process when finished
  analyzer.dispose();
}
```

Thrown errors inherit from `SemanticsError` and carry a `kind` string:
- `"syntax"`, `"empty_content"`, `"unsupported_language"`, `"file_too_large"`, `"binary_content"`, `"parse_failure"`, `"invalid_options"`, `"canceled"`, `"internal"`, `"backend_unavailable"`.



## Stability Guarantees

- **JSON Stability:** The output structure and its `snake_case` JSON field names are completely frozen.
- **API Stability:** Because `coach` is currently pre-1.0, the core JSON structure and sentinel errors are stable, but other parts of the API surface may evolve.
