package coachapi_test

import (
	"strings"

	"github.com/lousy-agents/coach/internal/coachapi"

	"github.com/lousy-agents/coach/internal/rubrics"
)

func countFindingsBySource(findings []coachapi.JobFinding) (det, agent, detHidden, agentHidden int) {
	for _, f := range findings {
		switch f.Source {
		case coachapi.FindingSourceDeterministic:
			det++
			if strings.Contains(string(f.Payload), "hidden_input_mutation") ||
				strings.Contains(string(f.Payload), "state.hidden_input_mutation") {
				detHidden++
			}
		case coachapi.FindingSourceAgent:
			agent++
			if f.RubricID != nil && *f.RubricID == rubrics.IDHiddenMutationContextualization {
				agentHidden++
			}
		}
	}
	return det, agent, detHidden, agentHidden
}
