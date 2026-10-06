package coachapi

import (
	"encoding/json"
)

// SummarizeFindings groups findings by source then rule id (deterministic,
// parsed from the finding payload's rule_id field) or rubric id (agent,
// from the finding's own RubricID) so counts cannot collide across sources.
func SummarizeFindings(findings []JobFinding) ReportSummary {
	counts := map[string]map[string]int{}
	for _, f := range findings {
		key := findingSummaryKey(f)
		if key == "" {
			continue
		}
		source := string(f.Source)
		if counts[source] == nil {
			counts[source] = map[string]int{}
		}
		counts[source][key]++
	}
	return ReportSummary{FindingCounts: counts}
}

func findingSummaryKey(f JobFinding) string {
	if f.Source == FindingSourceAgent {
		if f.RubricID != nil {
			return *f.RubricID
		}
		return ""
	}
	return findingRuleID(f.Payload)
}

func findingRuleID(payload json.RawMessage) string {
	var decoded struct {
		RuleID string `json:"rule_id"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return ""
	}
	return decoded.RuleID
}

func ReportFindings(jobFindings []JobFinding) []Finding {
	out := make([]Finding, len(jobFindings))
	for i, f := range jobFindings {
		out[i] = Finding{
			Source:        f.Source,
			RubricID:      clonePtrString(f.RubricID),
			RubricVersion: clonePtrString(f.RubricVersion),
			ModelIdentity: clonePtrString(f.ModelIdentity),
			Payload:       cloneRawMessage(f.Payload),
		}
	}
	return out
}

func ReportDiagnostics(jobDiagnostics []JobDiagnostic) []Diagnostic {
	out := make([]Diagnostic, len(jobDiagnostics))
	for i, d := range jobDiagnostics {
		out[i] = Diagnostic{Scope: d.Scope, Message: d.Message}
	}
	return out
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}

func clonePtrString(p *string) *string {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
