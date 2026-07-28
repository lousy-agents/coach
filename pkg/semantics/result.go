package semantics

// Result is the top-level output of analyzing one source file.
type Result struct {
	Path                string                        `json:"path"`
	Language            Language                      `json:"language"`
	ParseStatus         ParseStatus                   `json:"parse_status"`
	SyntaxErrors        []SyntaxIssue                 `json:"syntax_errors,omitempty"`
	Imports             []ImportFeature               `json:"imports,omitempty"`
	Metrics             StructuralMetrics             `json:"metrics"`
	Findings            []Finding                     `json:"findings,omitempty"`
	CognitiveComplexity []FunctionCognitiveComplexity `json:"cognitive_complexity,omitempty"`
	// ReactComponents holds per-component React orchestration facts.
	// AnalyzeBytes populates this field for LanguageTypeScript and
	// LanguageTSX only; it stays nil for Go. Entries are ordered by
	// location.start_byte ascending, then by name.
	ReactComponents []ReactComponentFacts `json:"react_components,omitempty"`
}

// ReactComponentFacts describes one detected React component's state and
// coordination shape, used by the react_component_orchestration_density
// codesignal rule. AnalyzeBytes populates Name, Location, ClientKind,
// UseState, CoordinatedTransitions, WorkspaceBranches, ImperativeUI, and
// SharedPanelDeps for TypeScript/TSX.
type ReactComponentFacts struct {
	Name                   string                       `json:"name"`
	Location               Location                     `json:"location"`
	ClientKind             string                       `json:"client_kind"`
	UseState               []ReactUseStateBinding       `json:"use_state,omitempty"`
	CoordinatedTransitions []ReactCoordinatedTransition `json:"coordinated_transitions,omitempty"`
	WorkspaceBranches      []ReactWorkspaceBranch       `json:"workspace_branches,omitempty"`
	ImperativeUI           []ReactImperativeUICall      `json:"imperative_ui,omitempty"`
	SharedPanelDeps        []ReactSharedPanelDep        `json:"shared_panel_deps,omitempty"`
}

// ReactUseStateBinding is one useState() call's binding/setter pair.
type ReactUseStateBinding struct {
	Binding  string   `json:"binding"`
	Setter   string   `json:"setter"`
	Location Location `json:"location"`
}

// ReactCoordinatedTransition is a callback (e.g. a useEffect body or event
// handler) that updates more than one state binding together.
type ReactCoordinatedTransition struct {
	Name            string   `json:"name"`
	Kind            string   `json:"kind"`
	Location        Location `json:"location"`
	UpdatedBindings []string `json:"updated_bindings"`
}

// ReactWorkspaceBranch is one branch of a multi-way conditional that renders
// a distinct panel/view.
type ReactWorkspaceBranch struct {
	Label    string   `json:"label"`
	Location Location `json:"location"`
}

// ReactImperativeUICall is a call to an imperative DOM/UI API (e.g.
// document.getElementById, .focus()) inside a component body.
type ReactImperativeUICall struct {
	API      string   `json:"api"`
	Location Location `json:"location"`
}

// ReactSharedPanelDep is a state binding passed as a prop to more than one
// distinct child panel component.
type ReactSharedPanelDep struct {
	Name   string   `json:"name"`
	Panels []string `json:"panels"`
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
	Ifs                    int `json:"ifs"`
	Fors                   int `json:"fors"`
	ExprSwitches           int `json:"expr_switches"`
	TypeSwitches           int `json:"type_switches"`
	Selects                int `json:"selects"`
	Functions              int `json:"functions"`
	Methods                int `json:"methods"`
	MaxNestingDepth        int `json:"max_nesting_depth"`
	MaxCognitiveComplexity int `json:"max_cognitive_complexity"`
	SumCognitiveComplexity int `json:"sum_cognitive_complexity"`
}

// FunctionCognitiveComplexity is the Cognitive Complexity score for one
// scored function body (declaration, method, func lit, or arrow).
type FunctionCognitiveComplexity struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Location Location `json:"location"`
	Score    int      `json:"score"`
}

// Finding describes one detected pattern of interest, such as a
// constructor-like function. Confidence, Evidence, Recommendation, and
// SuggestedSkill are optional coaching metadata used by findings like
// "mutates_input"; they're omitted (via omitempty) for findings that don't
// set them (e.g. "constructor_func", "pointer_return", "tight_coupling").
type Finding struct {
	Kind           string   `json:"kind"` // "constructor_func" | "pointer_return" | "mutates_input" (Go); "tight_coupling" | "mutates_input" (TS/TSX)
	Name           string   `json:"name"`
	Location       Location `json:"location"`
	Confidence     string   `json:"confidence,omitempty"`
	Evidence       string   `json:"evidence,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
	SuggestedSkill string   `json:"suggested_skill,omitempty"`
}
