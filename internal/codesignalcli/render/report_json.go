package render

import (
	"encoding/json"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// ReportJSON renders report as its canonical JSON representation followed
// by exactly one trailing newline, with no CLI-only wrapper fields added.
func ReportJSON(report *codesignal.Report) ([]byte, error) {
	encoded, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
