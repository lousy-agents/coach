package coachapi

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

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
