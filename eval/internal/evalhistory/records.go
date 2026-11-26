package evalhistory

import (
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// Records turns a history run into one flagged_vs_unflagged_association
// record per placed finding: 1 with the fix's SHA when a fix up to
// history_until touched the finding's unit, else 0 with "none".
func Records(summary Summary, date string) []evalcensus.Record {
	var records []evalcensus.Record
	for _, repo := range summary.Repositories {
		for _, placed := range repo.findings {
			record := evalcensus.Record{
				Target:  findingTarget(summary.CorpusID, repo.Since, placed),
				Measure: "flagged_vs_unflagged_association",
				Source:  evalcensus.Source{Kind: "szz", Ref: "none"},
				Rater:   evalcensus.Rater{Kind: "automated"},
				Date:    date,
			}
			record.Value, record.Rationale = json.RawMessage("0"), fmt.Sprintf("%s. No fix up to %s touched this unit.", Statement, repo.Until)
			if placed.fix != "" {
				record.Value, record.Source.Ref = json.RawMessage("1"), placed.fix
				record.Rationale = fmt.Sprintf("%s. Fix %s, up to %s, touched this unit.", Statement, placed.fix, repo.Until)
			}
			record.RecordID = evalcensus.NewRecordID(record)
			records = append(records, record)
		}
	}
	return records
}

// findingTarget names the finding at the census SHA. A file-level finding
// has no line span.
func findingTarget(corpusID, sha string, placed placedFinding) evalcensus.Target {
	target := evalcensus.Target{CorpusID: corpusID, SHA: sha, RuleID: placed.RuleID, Path: placed.Path}
	if placed.Subject != "" {
		target.Subject = &placed.Subject
	}
	if placed.Fingerprint != "" {
		target.Fingerprint = &placed.Fingerprint
	}
	if placed.level == FunctionLevel {
		target.Lines = &[2]int{placed.StartRow + 1, placed.EndRow + 1}
	}
	return target
}
