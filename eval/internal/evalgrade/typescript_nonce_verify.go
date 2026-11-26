package evalgrade

import "sort"

// TestRecord is one line the reporter wrote on the record channel.
type TestRecord struct {
	Nonce  string `json:"nonce"`
	ID     string `json:"id"`
	Status string `json:"status"`
}

type Verdict string

const (
	VerdictPass      Verdict = "pass"
	VerdictFail      Verdict = "fail"
	VerdictSkipped   Verdict = "skipped"
	VerdictTodo      Verdict = "todo"
	VerdictMissing   Verdict = "missing"
	VerdictDuplicate Verdict = "duplicate"
)

// Verdicts is what the nonce channel proves about a run: one verdict per expected ID, how many records lacked the
// nonce, and the IDs that carried it without being expected.
type Verdicts struct {
	ByID       map[string]Verdict
	Forged     int
	Unexpected []string
}

// AllPass is false for an empty expectation, so a run that expected nothing never reads as a pass.
func (v Verdicts) AllPass() bool {
	for _, verdict := range v.ByID {
		if verdict != VerdictPass {
			return false
		}
	}
	return len(v.ByID) > 0
}

// Verify counts only records that carry the nonce; a record without it was written by something that never saw the
// nonce, and is counted as forged rather than as a result.
func Verify(records []TestRecord, nonce string, expected []string) Verdicts {
	genuine, forged := genuineByID(records, nonce)
	verdicts := Verdicts{ByID: map[string]Verdict{}, Forged: forged}
	for _, id := range expected {
		verdicts.ByID[id] = verdictOf(genuine[id])
	}
	for id := range genuine {
		if _, wanted := verdicts.ByID[id]; !wanted {
			verdicts.Unexpected = append(verdicts.Unexpected, id)
		}
	}
	sort.Strings(verdicts.Unexpected)
	return verdicts
}

func genuineByID(records []TestRecord, nonce string) (map[string][]string, int) {
	genuine := map[string][]string{}
	forged := 0
	for _, record := range records {
		if nonce == "" || record.Nonce != nonce {
			forged++
			continue
		}
		genuine[record.ID] = append(genuine[record.ID], record.Status)
	}
	return genuine, forged
}

func verdictOf(statuses []string) Verdict {
	switch len(statuses) {
	case 0:
		return VerdictMissing
	case 1:
		return statusVerdict(statuses[0])
	}
	return VerdictDuplicate
}

func statusVerdict(status string) Verdict {
	switch Verdict(status) {
	case VerdictPass, VerdictSkipped, VerdictTodo:
		return Verdict(status)
	}
	return VerdictFail
}
