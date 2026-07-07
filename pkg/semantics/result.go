package semantics

// Result is the top-level output of analyzing one source file.
type Result struct {
	Path         string            `json:"path"`
	Language     Language          `json:"language"`
	ParseStatus  ParseStatus       `json:"parse_status"`
	SyntaxErrors []SyntaxIssue     `json:"syntax_errors,omitempty"`
	Imports      []ImportFeature   `json:"imports,omitempty"`
	Metrics      StructuralMetrics `json:"metrics"`
	Findings     []Finding         `json:"findings,omitempty"`
}

// Location is a 0-based byte/row/col span as Tree-sitter reports it.
type Location struct {
	StartByte uint `json:"start_byte"`
	EndByte   uint `json:"end_byte"`
	StartRow  uint `json:"start_row"`
	StartCol  uint `json:"start_col"`
	EndRow    uint `json:"end_row"`
	EndCol    uint `json:"end_col"`
}

// SyntaxIssue describes one syntax error or missing-node location found
// while parsing.
type SyntaxIssue struct {
	Kind     string   `json:"kind"` // "error" | "missing"
	Location Location `json:"location"`
}

// ImportFeature describes one import declaration.
type ImportFeature struct {
	Path     string   `json:"path"`
	Alias    string   `json:"alias,omitempty"` // alias ident, ".", or "_"
	Location Location `json:"location"`
}

// StructuralMetrics counts branching/declaration constructs across a file.
// TypeSwitches and Selects have no TypeScript/TSX analog and are always 0
// for those languages (Go-only fields).
type StructuralMetrics struct {
	Ifs             int `json:"ifs"`
	Fors            int `json:"fors"`
	ExprSwitches    int `json:"expr_switches"`
	TypeSwitches    int `json:"type_switches"`
	Selects         int `json:"selects"`
	Functions       int `json:"functions"`
	Methods         int `json:"methods"`
	MaxNestingDepth int `json:"max_nesting_depth"`
}

// Finding describes one detected pattern of interest, such as a
// constructor-like function.
type Finding struct {
	Kind     string   `json:"kind"` // "constructor_func" | "pointer_return" (Go); "tight_coupling" (TS/TSX)
	Name     string   `json:"name"`
	Location Location `json:"location"`
}
