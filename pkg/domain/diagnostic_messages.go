package domain

// defaultDiagnosticMessages is what a coverage diagnostic says when the code that raised it gave no message of its
// own: what was found and, where the user can act, what to do. A report must never show a diagnostic code alone.
var defaultDiagnosticMessages = map[string]string{
	DiagBackendUnavailable:  "the project analysis backend could not be started, so no project findings were produced; install or repair the toolchain the project language needs, then re-run",
	DiagRootScopeIncomplete: "a TypeScript root's analysed files do not cover every candidate file, so findings for the files left out are missing; check the root's tsconfig include and exclude",

	DiagLayerBypassAmbiguousLayer:         "the policy's required_layer could not be applied: no function or file the layer-bypass search follows lies under its prefixes (for Go, a layer whose packages declare only types has none), or a function on a path the search followed has no source position, so its layer could not be decided (a limit of the analysis, not of the policy), and no layer-bypass witness is reported for this run; for the first cause, check the layer's prefixes or add code under them; the second needs no policy change, so treat this run's layer-bypass result as incomplete",
	DiagLayerBypassBudgetExceeded:         "the layer-bypass search did not run to completion, because its budget or deadline was reached or because the required layer could not be applied (see the other layer-bypass diagnostics), so the absence of a witness does not mean there is no bypass; narrow the policy's roots, or re-run when the machine is less loaded",
	DiagLayerBypassRequiredLayerUnmatched: "the policy's required_layer matched no package in the analysed program: correct its prefixes in the policy, or add the layer",
	DiagLayerBypassSourceLoadFailed:       "the Go sources of a root could not be loaded for the layer-bypass search; fix the build errors in that root and re-run",

	DiagReachabilityBudgetExceeded:   "the reachability search stopped at its budget or deadline, so the absence of a path does not mean there is none; narrow the policy's roots, or re-run when the machine is less loaded",
	DiagReachabilitySourceLoadFailed: "the Go sources of a root could not be loaded for the reachability search; fix the build errors in that root and re-run",

	"project_callgraph_budget_exceeded":              "the call-graph build stopped at its budget or deadline, so call facts are partial; narrow the policy's roots, or re-run when the machine is less loaded",
	"project_callgraph_build_failed":                 "the Go sources of a root could not be loaded or type-checked, so no call-graph facts were produced for it; fix the build errors in that root and re-run",
	"project_call_unresolved_interface":              "a call through an interface method has no statically known callee, so it contributes no call fact",
	"project_call_unresolved_function_value":         "a call through a function-typed variable, field or parameter has no statically known callee, so it contributes no call fact",
	"project_call_unresolved_reflection":             "a call dispatched through reflection has no statically known callee, so it contributes no call fact",
	"project_call_unresolved_framework_registration": "a handler value was passed to an HTTP registration call instead of being invoked, so the framework's calls to it are not followed",
	"project_call_unresolved_synthetic_wrapper":      "a call reaches a compiler-generated wrapper (a bound method value or a promoted method) whose real target is in the snapshot, so the route through it is not followed",
	"project_file_budget_exceeded":                   "the file budget was reached before every file was read, so imports from the remaining files are missing; narrow the policy's roots",
	"project_file_syntax_error":                      "a Go file does not parse, so its imports were not read; fix the syntax error",
	"project_file_unavailable":                       "a Go file could not be read from the snapshot, so its imports were not read",
	"project_root_unavailable":                       "a module root could not be read, so nothing under it was analysed; check that the path exists in the repository",
	"project_root_outside_snapshot":                  "a go.work use directive names a directory outside the snapshot, so it was not analysed",
	"project_root_invalid":                           "a module root is not a valid Go module root, so nothing under it was analysed; check its go.mod",
	"project_root_duplicate":                         "a module root is listed more than once, so it was analysed once; remove the duplicate from the policy or go.work",
	"project_root_ambiguous":                         "a module root is named in more than one way, so which module owns its packages is ambiguous; name it one way in the policy or go.work",
	"project_root_incomplete":                        "module root discovery stopped before it had walked every directory, so some roots may be missing; narrow the policy's roots",
	"project_roots_empty":                            "the policy's roots held no source file to analyse, so a clean result says nothing; check the roots in the policy",
	"ts_project_root_unavailable":                    "a TypeScript project root could not be read, so nothing under it was analysed; check that the path exists in the repository",
	"ts_project_root_incomplete":                     "TypeScript root discovery stopped before it had walked every directory, so some roots may be missing; narrow the policy's roots",
}

// DefaultDiagnosticMessage is the message a diagnostic with the given code carries when it has none of its own, or
// "" for a code the table does not hold.
func DefaultDiagnosticMessage(code string) string { return defaultDiagnosticMessages[code] }
