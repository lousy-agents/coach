# coach

experimental ai coach for humans making software with agents

## Packages

- [`pkg/semantics`](./pkg/semantics) — deterministic structural analysis of raw Go source bytes (syntax validity, imports, branching metrics, constructor-like patterns) via Tree-sitter, plus coaching findings like `mutates_input` (see [Coaching findings](#coaching-findings) below). No GitHub dependency.
- [`pkg/githubingest`](./pkg/githubingest) — optional GitHub App-authenticated file reader. Never imported by `pkg/semantics`, and never imports it back.

## Install

```sh
go get github.com/lousy-agents/coach/pkg/semantics
go get github.com/lousy-agents/coach/pkg/githubingest # only if you need GitHub App file fetching
```

### CGO requirement

By default `pkg/semantics` binds to Tree-sitter's C runtime via `github.com/tree-sitter/go-tree-sitter`. It requires `CGO_ENABLED=1` and a C toolchain (e.g. `gcc`) at build time. `pkg/githubingest` has no such requirement.

When CGO is unavailable — `CGO_ENABLED=0`, or `GOOS=js GOARCH=wasm`, which cannot use CGO at all — `pkg/semantics` automatically falls back to a pure-Go engine ([`github.com/odvcencio/gotreesitter`](https://github.com/odvcencio/gotreesitter)), with no code or flag changes required. This fallback is newer than the CGO engine and is verified against the fixture corpus in `pkg/semantics/backend_conformance_test.go`, not proven identical to the CGO engine on every possible malformed input — see that file and `pkg/semantics/doc.go` for details. A `coach_gotreesitter` build tag forces the pure-Go engine on a native (CGO-capable) build, for testing or comparison. `mise run wasm-build` proves a real `GOOS=js GOARCH=wasm` build compiles; `mise run conformance-test` runs the dual-backend suite.

## `pkg/semantics` quickstart

```go
analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
if err != nil {
    // ...
}

result, err := analyzer.AnalyzeBytes(ctx, semantics.FileInput{
    Path:     "greeter.go",
    Language: semantics.LanguageGo,
    Content:  sourceBytes,
})
if err != nil {
    // handle below
}

fmt.Println(result.ParseStatus) // "ok" or "syntax_errors"
for _, f := range result.Findings {
    fmt.Println(f.Kind, f.Name)
}
```

See `pkg/semantics/example_test.go` (`ExampleAnalyzer_AnalyzeBytes`) for a runnable version of this snippet.

### Error matching

Syntax errors return a partial `*Result` (`ParseStatus == "syntax_errors"`) alongside an error you can match with `errors.Is`/`errors.As`:

```go
result, err := analyzer.AnalyzeBytes(ctx, in)
if errors.Is(err, semantics.ErrSyntax) {
    var syntaxErr *semantics.SyntaxError
    if errors.As(err, &syntaxErr) {
        for _, issue := range syntaxErr.Issues {
            fmt.Println(issue.Kind, issue.Location)
        }
    }
}
```

Other sentinels: `ErrEmptyContent`, `ErrUnsupportedLanguage`, `ErrFileTooLarge`, `ErrBinaryContent`, `ErrParseFailure`. See `pkg/semantics/example_test.go` (`ExampleAnalyzer_AnalyzeBytes_syntaxError`) for a runnable version.

### Coaching findings

Beyond raw metrics, `result.Findings` can carry coaching-oriented findings: `constructor_func` and `pointer_return` (Go), `tight_coupling` (TS/TSX), and `mutates_input` (Go, TS, TSX).

`mutates_input` flags a function/method writing through its own parameter in a way that's visible to the caller after the call returns — a common source of confusing "spooky action at a distance" bugs. For Go, this means a selector or dereference write through a pointer-typed parameter (`cfg.Name = x`, `(*cfg).Name = x`) or an index write on a map/slice-typed parameter (`values[k] = x`, `items[i] = x`). For TS/TSX, this means a property/index assignment or `delete` on an identifier-bound parameter, or a call to one of a fixed set of known in-place-mutating methods (`copyWithin`, `fill`, `pop`, `push`, `reverse`, `shift`, `sort`, `splice`, `unshift`, `set`, `add`, `delete`, `clear`) on one.

```go
func ApplyDefaults(cfg *Config) {
    cfg.Timeout = 30 * time.Second // mutates the caller's Config in place
}
```

produces a finding shaped like:

```json
{
  "kind": "mutates_input",
  "name": "ApplyDefaults:cfg",
  "confidence": "medium",
  "evidence": "cfg.Timeout = 30 * time.Second",
  "recommendation": "Return a copy instead of mutating the caller's value, or document/rename this function to make the in-place mutation explicit.",
  "suggested_skill": "refactor-hidden-mutation"
}
```

vs. the caller-safe alternative, which returns a new value instead of writing through the parameter:

```go
func WithDefaults(cfg Config) Config {
    cfg.Timeout = 30 * time.Second
    return cfg
}
```

`mutates_input` is deliberately conservative and purely syntactic: it does not do whole-program alias analysis, does not track aliases assigned to local variables, does not infer types beyond a parameter's own syntactic declaration, does not follow values across function calls (no interprocedural dataflow), and — for TS/TSX — only recognizes the fixed built-in method list above, not arbitrary custom mutating methods, and does not track destructured, rest, or defaulted parameters at all.

The `confidence`, `evidence`, `recommendation`, and `suggested_skill` `Finding` fields are additive and `omitempty`; they're populated on `mutates_input` findings but absent (and unaffected) on the pre-existing `constructor_func`, `pointer_return`, and `tight_coupling` findings.

## Run locally: analyze a local repository

There is no `coach` CLI yet — `pkg/semantics` is a library today. To analyze the Go files in a local checkout, write a small program that walks the tree and calls `AnalyzeBytes` yourself:

```go
// main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: analyze <path-to-repo>")
		os.Exit(1)
	}
	root := os.Args[1]

	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
			Path:     path,
			Language: semantics.LanguageGo,
			Content:  content,
		})
		if err != nil && result == nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			return nil
		}

		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

Then, with a C toolchain available (see [CGO requirement](#cgo-requirement) above):

```sh
mkdir coach-analyze && cd coach-analyze
go mod init coach-analyze
go get github.com/lousy-agents/coach/pkg/semantics
# save the program above as main.go, then:
go run . /path/to/your/repo
```

This prints one JSON `Result` object per `.go` file found under `/path/to/your/repo`, in the shape documented under [JSON stability](#json-stability) below. Files that fail to parse still print a partial result (`parse_status: "syntax_errors"`) rather than being skipped.

## JS/TS usage (`js/semantics`)

[`js/semantics`](./js/semantics) packages `pkg/semantics` for Node.js as `@lousy-agents/coach-semantics` — a typed, ESM-only npm package. It is not published to npm: consume it by cloning this repository and building locally.

Under the hood the package talks newline-delimited JSON to a small Go binary (`cmd/semantics-json`) over stdin/stdout. A WebAssembly transport was the preferred design; it was originally blocked because `pkg/semantics` required CGO for Tree-sitter and standard `GOOS=js GOARCH=wasm` does not support CGO. `pkg/semantics` now also builds against a pure-Go engine (see [CGO requirement](#cgo-requirement) above and `pkg/semantics/doc.go`), so a real `GOOS=js GOARCH=wasm` build compiles and runs (`mise run wasm-build`, `cmd/semantics-wasm-smoke` for a runnable proof) — but `js/semantics` does not consume it yet. Wiring an actual WASM `Backend` implementation (`backend-wasm.ts`) to replace or complement the stdio child process is a separate, later decision; the transport is an implementation detail behind the package's `Backend` seam either way, so that swap won't change the public API.

Prerequisites: Node.js ≥ 20, plus the Go + C toolchain described under [CGO requirement](#cgo-requirement) (the backend binary is compiled from this repo).

```sh
git clone https://github.com/lousy-agents/coach.git
cd coach/js/semantics
npm install   # builds the Go backend binary and the TS package (prepare script)
```

Then depend on the directory from your app (`npm link`, or a `file:` dependency):

```sh
cd ~/your-app
npm install /path/to/coach/js/semantics
```

```ts
import { readFile } from "node:fs/promises";
import { createAnalyzer, SemanticsSyntaxError } from "@lousy-agents/coach-semantics";

const analyzer = await createAnalyzer();
try {
  const result = await analyzer.analyzeBytes({
    path: "widget.go",
    language: "go", // or "typescript" / "tsx"
    content: await readFile("widget.go"),
  });
  console.log(result.parse_status, result.metrics, result.findings);
} catch (err) {
  if (err instanceof SemanticsSyntaxError) {
    // Mirrors Go's double return: the partial Result rides on the error.
    console.log(err.partialResult.syntax_errors);
  } else {
    throw err;
  }
} finally {
  analyzer.dispose();
}
```

Results use the exact frozen `snake_case` JSON shape documented under [JSON stability](#json-stability); a parity test suite replays shared fixtures through both the Go API and the JS package to keep the two byte-identical. In place of `errors.Is`, thrown `SemanticsError`s carry a `kind` string (`"syntax"`, `"empty_content"`, `"unsupported_language"`, `"file_too_large"`, `"binary_content"`, `"parse_failure"`, `"invalid_options"`, `"canceled"`, `"internal"`, `"backend_unavailable"`).

Repo-side build/test tasks: `mise run backend-build`, `mise run js-build`, `mise run js-test` (all part of `mise run ci`).

## `pkg/githubingest` quickstart

```go
reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
    AppID:          123,
    InstallationID: 456,
    PrivateKey:     appPrivateKeyPEM, // as issued by GitHub, PKCS#1 PEM
})
if err != nil {
    // ...
}

content, meta, err := reader.ReadFile(ctx, githubingest.GitHubFileRef{
    Owner: "lousy-agents", Repo: "coach", Ref: "main", Path: "go.mod",
})
```

`ReadFile` maps API failures to sentinels: `ErrNotFound` (404), `ErrAuth` (401/403), `ErrUnsupportedContent` (directory/symlink/submodule), `ErrTooLarge` (>1 MiB), `ErrEmptyContent`. See `pkg/githubingest/example_test.go` (`ExampleNewGitHubFileReader`) for a runnable version.

Each `ReadFile` call issues two Contents API requests: the file fetch itself, plus a listing of its parent directory so an in-repo symlink (which GitHub's Contents API otherwise resolves transparently, reporting it as a plain file) is still detected and rejected. The second request is scoped to one directory, not a whole-repository tree, so its cost stays constant regardless of repository size. That listing is capped at GitHub's documented limit of 1,000 entries per directory; a symlink in a larger directory can go undetected, since the Contents API gives no truncation signal to check for.

## JSON stability

`Result` and its nested types carry frozen, `snake_case` JSON field names (see `pkg/semantics/result.go`). A golden-file test (`pkg/semantics/result_test.go`) locks the shape byte-for-byte.

## API stability

Both packages are pre-1.0. JSON field names and error identities (the sentinel `Err*` values and `*SyntaxError`) are treated as stable; other API surface may still change.
