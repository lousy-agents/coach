package codesignalcli

import (
	"encoding/json"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// RenderText is not yet implemented.
func RenderText(report *codesignal.Report) string {
	return ""
}

// RenderJSON is not yet implemented.
func RenderJSON(report *codesignal.Report) ([]byte, error) {
	return json.Marshal(report)
}
