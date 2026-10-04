package codesignal

import (
	"testing"
)

func TestDiagnostics_NoMissingHeadResultWhenNotModifiedOrAdded(t *testing.T) {
	for _, status := range []ChangeStatus{"removed", "unknown", ""} {
		t.Run("status="+string(status), func(t *testing.T) {
			body_diagnosticsHeadresultPart4Test_10(t, status)
		})
	}
}
