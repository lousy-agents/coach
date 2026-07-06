# coach

experimental ai coach for humans making software with agents

## Packages

- [`pkg/semantics`](./pkg/semantics) — deterministic structural analysis of raw Go source bytes (syntax validity, imports, branching metrics, constructor-like patterns) via Tree-sitter. No GitHub dependency.
- [`pkg/githubingest`](./pkg/githubingest) — optional GitHub App-authenticated file reader. Never imported by `pkg/semantics`, and never imports it back.

## Install

```sh
go get github.com/lousy-agents/coach/pkg/semantics
go get github.com/lousy-agents/coach/pkg/githubingest # only if you need GitHub App file fetching
```

### CGO requirement

`pkg/semantics` binds to Tree-sitter's C runtime via `github.com/tree-sitter/go-tree-sitter`. It requires `CGO_ENABLED=1` and a C toolchain (e.g. `gcc`) at build time — `CGO_ENABLED=0` builds of `pkg/semantics` are not possible. `pkg/githubingest` has no such requirement.

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

## JSON stability

`Result` and its nested types carry frozen, `snake_case` JSON field names (see `pkg/semantics/result.go`). A golden-file test (`pkg/semantics/result_test.go`) locks the shape byte-for-byte.

## API stability

Both packages are pre-1.0. JSON field names and error identities (the sentinel `Err*` values and `*SyntaxError`) are treated as stable; other API surface may still change.
