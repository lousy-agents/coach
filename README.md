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
