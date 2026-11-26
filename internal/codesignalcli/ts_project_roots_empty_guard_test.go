package codesignalcli

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestWithEmptyTSProjectsJudgedOnlyJudgesOtherwiseCompleteCoverageWithNoProject(t *testing.T) {
	truncated := projectmodel.Diagnostic{Code: projectmodel.DiagRootIncomplete, Message: "discovery stopped at the input-file budget"}
	rows := []struct {
		name         string
		coverage     projectmodel.Coverage
		wantJudged   bool
		wantComplete bool
	}{
		{
			name:         "complete coverage with no tsconfig is judged: the tsconfig clause alone decides",
			coverage:     projectmodel.Coverage{Complete: true, Counts: map[string]int{"tsconfig_count": 0}},
			wantJudged:   true,
			wantComplete: false,
		},
		{
			name:         "complete coverage with a tsconfig is left alone: the tsconfig clause alone decides",
			coverage:     projectmodel.Coverage{Complete: true, Counts: map[string]int{"tsconfig_count": 1}},
			wantJudged:   false,
			wantComplete: true,
		},
		{
			name:         "incomplete coverage with no tsconfig keeps its own cause: the completeness clause alone decides",
			coverage:     projectmodel.Coverage{Complete: false, Counts: map[string]int{"tsconfig_count": 0}, Diagnostics: []projectmodel.Diagnostic{truncated}},
			wantJudged:   false,
			wantComplete: false,
		},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			got := withEmptyTSProjectsJudged(row.coverage, []string{"."})

			named := false
			for _, diagnostic := range got.Diagnostics {
				named = named || diagnostic.Code == projectmodel.DiagRootsEmpty
			}
			if named != row.wantJudged {
				t.Fatalf("project_roots_empty named = %v, want %v; diagnostics %+v", named, row.wantJudged, got.Diagnostics)
			}
			if got.Complete != row.wantComplete {
				t.Fatalf("Complete = %v, want %v", got.Complete, row.wantComplete)
			}
			if !row.wantJudged && !reflect.DeepEqual(got, row.coverage) {
				t.Fatalf("coverage changed: got %+v, want %+v", got, row.coverage)
			}
		})
	}
}
