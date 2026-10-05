package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// TestSuggestPrimaryRootDiagnostic covers the diagnostic-code mapping and
// priority ordering from issue #220 that are impractical to trigger via a
// real Git repository at the acceptance-test level (root-discovery
// budget exhaustion, an unavailable snapshot, and multiple simultaneous
// diagnostic severities), per project_config_suggestion_acceptance_test.go
// covering the remaining rows end-to-end through the coach binary.
func TestSuggestPrimaryRootDiagnostic(t *testing.T) {
	tests := []struct {
		name     string
		result   projectmodel.RootDiscoveryResult
		wantCode string
		wantOK   bool
		wantPath string
	}{
		{
			name: "unavailable takes priority over everything else",
			result: projectmodel.RootDiscoveryResult{
				Complete: false,
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{
						{Code: projectmodel.DiagRootAmbiguous, Path: "nested"},
						{Code: projectmodel.DiagRootUnavailable, Path: "."},
					},
				},
			},
			wantCode: SuggestDiagSnapshotUnavailable,
			wantPath: ".",
		},
		{
			name: "outside_snapshot maps to ambiguous_roots",
			result: projectmodel.RootDiscoveryResult{
				Complete: true,
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{{Code: projectmodel.DiagRootOutsideSnapshot, Path: "../outside"}},
				},
			},
			wantCode: SuggestDiagAmbiguousRoots,
			wantPath: "../outside",
		},
		{
			name: "invalid maps to ambiguous_roots",
			result: projectmodel.RootDiscoveryResult{
				Complete: true,
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{{Code: projectmodel.DiagRootInvalid, Path: "bad/go.mod", Message: "parse error"}},
				},
			},
			wantCode: SuggestDiagAmbiguousRoots,
			wantPath: "bad/go.mod",
		},
		{
			name: "duplicate maps to ambiguous_roots",
			result: projectmodel.RootDiscoveryResult{
				Complete: true,
				Roots:    []string{"dup"},
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{{Code: projectmodel.DiagRootDuplicate, Path: "dup"}},
				},
			},
			wantCode: SuggestDiagAmbiguousRoots,
			wantPath: "dup",
		},
		{
			name: "ambiguous maps to ambiguous_roots",
			result: projectmodel.RootDiscoveryResult{
				Complete: true,
				Roots:    []string{"nested", "nested/inner"},
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{{Code: projectmodel.DiagRootAmbiguous, Path: "nested"}},
				},
			},
			wantCode: SuggestDiagAmbiguousRoots,
			wantPath: "nested",
		},
		{
			name: "ambiguous-family diagnostics outrank incomplete",
			result: projectmodel.RootDiscoveryResult{
				Complete: false,
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{
						{Code: projectmodel.DiagRootIncomplete},
						{Code: projectmodel.DiagRootDuplicate, Path: "dup"},
					},
				},
			},
			wantCode: SuggestDiagAmbiguousRoots,
			wantPath: "dup",
		},
		{
			name: "incomplete with an explicit DiagRootIncomplete diagnostic",
			result: projectmodel.RootDiscoveryResult{
				Complete: false,
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{{Code: projectmodel.DiagRootIncomplete}},
				},
			},
			wantCode: SuggestDiagIncomplete,
		},
		{
			name: "incomplete with no specific DiagRootIncomplete diagnostic still maps to incomplete",
			result: projectmodel.RootDiscoveryResult{
				Complete: false,
			},
			wantCode: SuggestDiagIncomplete,
		},
		{
			name: "a complete but empty root set maps to no_go_modules",
			result: projectmodel.RootDiscoveryResult{
				Complete: true,
				Roots:    nil,
			},
			wantCode: SuggestDiagNoGoModules,
		},
		{
			name: "a complete, non-empty root set is ok",
			result: projectmodel.RootDiscoveryResult{
				Complete: true,
				Roots:    []string{".", "services/payments"},
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_projectConfigSuggestionPart3Test_133(t, tt)
		})
	}
}
