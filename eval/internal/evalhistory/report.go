package evalhistory

import (
	"encoding/json"
	"errors"
)

// finding is a Coach finding with the fingerprint its record carries.
type finding struct {
	Signal
	Fingerprint string
}

type reportSignal struct {
	RuleID      string `json:"rule_id"`
	Path        string `json:"path"`
	Subject     string `json:"subject"`
	Fingerprint string `json:"fingerprint"`
	Location    struct {
		StartRow int `json:"start_row"`
		EndRow   int `json:"end_row"`
	} `json:"location"`
}

var errUnreadableReport = errors.New("Coach's report could not be read as a JSON report with signals")

func decodeFindings(report []byte) ([]finding, error) {
	var decoded struct {
		Signals *[]reportSignal `json:"signals"`
	}
	if err := json.Unmarshal(report, &decoded); err != nil || decoded.Signals == nil {
		return nil, errUnreadableReport
	}
	findings := make([]finding, 0, len(*decoded.Signals))
	for _, signal := range *decoded.Signals {
		findings = append(findings, finding{
			Signal:      Signal{RuleID: signal.RuleID, Path: signal.Path, Subject: signal.Subject, StartRow: signal.Location.StartRow, EndRow: signal.Location.EndRow},
			Fingerprint: signal.Fingerprint,
		})
	}
	return findings, nil
}

func signalsOf(findings []finding) []Signal {
	signals := make([]Signal, len(findings))
	for i, finding := range findings {
		signals[i] = finding.Signal
	}
	return signals
}
