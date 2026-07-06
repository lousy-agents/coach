package semantics

// Language identifies the source language a Result was produced from.
type Language string

// LanguageGo is the only supported Language in v1.
const LanguageGo Language = "go"

// ParseStatus summarizes whether Tree-sitter found any syntax errors while
// parsing the source. The only values that exist in v1 are "ok" and
// "syntax_errors".
type ParseStatus string
