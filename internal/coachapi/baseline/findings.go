package baseline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func findingsFromCodeSignalReport(report *codesignal.Report) []coachapi.JobFinding {
	if report == nil {
		return nil
	}
	out := make([]coachapi.JobFinding, 0, len(report.Signals))
	for _, sig := range report.Signals {
		payload, err := json.Marshal(sig)
		if err != nil {
			continue
		}
		hash := sig.Fingerprint
		if hash == "" {
			hash = stablePayloadHash("deterministic", sig.RuleID, sig.Path, sig.Subject, sig.Evidence)
		}
		out = append(out, coachapi.JobFinding{
			ID:          watermill.NewUUID(),
			Source:      coachapi.FindingSourceDeterministic,
			Payload:     payload,
			PayloadHash: hash,
		})
	}
	return out
}

func diagnosticsFromCodeSignal(report *codesignal.Report) []coachapi.JobDiagnostic {
	if report == nil || len(report.Diagnostics) == 0 {
		return nil
	}
	out := make([]coachapi.JobDiagnostic, 0, len(report.Diagnostics))
	for _, d := range report.Diagnostics {
		scope := "codesignal"
		if d.Kind != "" {
			scope = "codesignal:" + d.Kind
		}
		msg := d.Message
		if d.Path != "" {
			msg = d.Path + ": " + msg
		}
		out = append(out, coachapi.JobDiagnostic{
			ID:      watermill.NewUUID(),
			Scope:   scope,
			Message: msg,
		})
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
