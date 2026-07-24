package coachapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/rubrics"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// jobOutcomeFromRubricTool maps a rubric tool envelope to a JobFinding or
// JobDiagnostic. hashDiscriminators are mixed into PayloadHash after the
// judgment payload so multiple judgments that share an identical ToolResult
// (common with stub/live canned output) remain unique under the store UNIQUE
// constraint — pass the deterministic finding's PayloadHash for per-signal
// hidden_mutation_contextualization calls.
func jobOutcomeFromRubricTool(raw json.RawMessage, hashDiscriminators ...string) (*JobFinding, *JobDiagnostic, error) {
	var tr rubrics.ToolResult
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, nil, fmt.Errorf("coachapi: decoding rubric tool result: %w", err)
	}
	if tr.Diagnostic != nil {
		return nil, &JobDiagnostic{
			ID:      watermill.NewUUID(),
			Scope:   tr.Diagnostic.Scope,
			Message: tr.Diagnostic.Message,
		}, nil
	}
	if !tr.HasJudgment() {
		return nil, &JobDiagnostic{
			ID:      watermill.NewUUID(),
			Scope:   "rubric:" + tr.RubricID,
			Message: "judgment failed: empty result",
		}, nil
	}
	payload, err := json.Marshal(tr)
	if err != nil {
		return nil, nil, err
	}
	rubricID := tr.RubricID
	rubricVersion := tr.RubricVersion
	var modelID *string
	if tr.ModelIdentity != nil {
		modelID = tr.ModelIdentity
	}
	hashParts := make([]string, 0, 3+len(hashDiscriminators))
	hashParts = append(hashParts, "agent", rubricID, string(payload))
	hashParts = append(hashParts, hashDiscriminators...)
	return &JobFinding{
		ID:            watermill.NewUUID(),
		Source:        FindingSourceAgent,
		RubricID:      &rubricID,
		RubricVersion: &rubricVersion,
		ModelIdentity: modelID,
		Payload:       payload,
		PayloadHash:   stablePayloadHash(hashParts...),
	}, nil, nil
}

func findingsFromCodeSignalReport(report *codesignal.Report) []JobFinding {
	if report == nil {
		return nil
	}
	out := make([]JobFinding, 0, len(report.Signals))
	for _, sig := range report.Signals {
		payload, err := json.Marshal(sig)
		if err != nil {
			continue
		}
		hash := sig.Fingerprint
		if hash == "" {
			hash = stablePayloadHash("deterministic", sig.RuleID, sig.Path, sig.Subject, sig.Evidence)
		}
		out = append(out, JobFinding{
			ID:          watermill.NewUUID(),
			Source:      FindingSourceDeterministic,
			Payload:     payload,
			PayloadHash: hash,
		})
	}
	return out
}

func diagnosticsFromCodeSignal(report *codesignal.Report) []JobDiagnostic {
	if report == nil || len(report.Diagnostics) == 0 {
		return nil
	}
	out := make([]JobDiagnostic, 0, len(report.Diagnostics))
	for _, d := range report.Diagnostics {
		scope := "codesignal"
		if d.Kind != "" {
			scope = "codesignal:" + d.Kind
		}
		msg := d.Message
		if d.Path != "" {
			msg = d.Path + ": " + msg
		}
		out = append(out, JobDiagnostic{
			ID:      watermill.NewUUID(),
			Scope:   scope,
			Message: msg,
		})
	}
	return out
}

func seedRubricVersions() map[string]string {
	seed := rubrics.Seed()
	out := make(map[string]string, len(seed))
	for _, def := range seed {
		out[def.ID] = def.Version
	}
	return out
}

func stablePayloadHash(parts ...string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			_, _ = h.Write([]byte{0})
		}
		_, _ = h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}
