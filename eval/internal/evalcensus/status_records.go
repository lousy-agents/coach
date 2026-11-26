package evalcensus

import (
	"encoding/json"
	"fmt"
)

// StatusRecords turns a census run into one analysis_status record per
// repository and rule. Each targets the repository root as a whole, and its
// rationale carries the counts or the failure evidence.
func StatusRecords(summary Summary, date string) []Record {
	var records []Record
	for _, repo := range summary.Repositories {
		for _, status := range repo.Statuses {
			record := Record{
				Target:    Target{CorpusID: summary.CorpusID, SHA: repo.SHA, RuleID: status.RuleID, Path: "."},
				Measure:   "analysis_status",
				Value:     json.RawMessage(fmt.Sprintf("%q", status.Status)),
				Source:    Source{Kind: "census", Ref: summary.Coach.SHA256},
				Rater:     Rater{Kind: "automated"},
				Date:      date,
				Rationale: statusRationale(status),
			}
			record.RecordID = NewRecordID(record)
			records = append(records, record)
		}
	}
	return records
}

func statusRationale(status RuleStatus) string {
	if status.Evidence != "" {
		return status.Evidence
	}
	return fmt.Sprintf("findings=%d adjudicable=%d sampled=%d", status.Findings, status.Adjudicable, len(status.SampledKeys))
}
