package memory

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/coachapi"
)

func cloneJob(j coachapi.Job) coachapi.Job {
	out := j
	out.Params = cloneRawMessage(j.Params)
	out.Error = cloneOptional(j.Error)
	out.StartedAt = cloneOptional(j.StartedAt)
	out.FinishedAt = cloneOptional(j.FinishedAt)
	out.ClaimedBy = cloneOptional(j.ClaimedBy)
	out.HeartbeatAt = cloneOptional(j.HeartbeatAt)
	return out
}

func cloneJobFindings(findings []coachapi.JobFinding) []coachapi.JobFinding {
	if findings == nil {
		return nil
	}
	out := make([]coachapi.JobFinding, len(findings))
	for i, f := range findings {
		out[i] = f
		out[i].RubricID = cloneOptional(f.RubricID)
		out[i].RubricVersion = cloneOptional(f.RubricVersion)
		out[i].ModelIdentity = cloneOptional(f.ModelIdentity)
		out[i].Payload = cloneRawMessage(f.Payload)
	}
	return out
}

func cloneJobDiagnostics(diagnostics []coachapi.JobDiagnostic) []coachapi.JobDiagnostic {
	if diagnostics == nil {
		return nil
	}
	out := make([]coachapi.JobDiagnostic, len(diagnostics))
	copy(out, diagnostics)
	return out
}

func cloneVersions(v coachapi.ReportVersions) coachapi.ReportVersions {
	out := v
	if v.Rubrics != nil {
		out.Rubrics = make(map[string]string, len(v.Rubrics))
		for k, val := range v.Rubrics {
			out.Rubrics[k] = val
		}
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

// cloneOptional copies the value behind an optional (nil-able) field so the
// store never shares a pointer with its caller.
func cloneOptional[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
