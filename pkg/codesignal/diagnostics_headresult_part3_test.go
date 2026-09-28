package codesignal

import (
	"testing"
)

func TestDiagnostics_MissingHeadResultOnModifiedOrAdded(t *testing.T) {
	for _, status := range []ChangeStatus{"modified", "added"} {
		t.Run(string(status), func(t *testing.T) {
			body_diagnosticsHeadresultPart3Test_10(t, status)
		})
	}
}
