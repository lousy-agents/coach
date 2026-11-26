package evalcensus

import "reflect"

// linkSupersessions makes each new record that re-measures a target the ledger holds supersede the latest record
// still current for that target, measure and source kind. A record is current until a later record supersedes it, and
// an earlier record is never rewritten. A record that already names what it supersedes is left as it is.
func linkSupersessions(existing, batch []Record) []Record {
	superseded := make(map[string]bool, len(existing))
	for _, record := range existing {
		if record.Supersedes != nil {
			superseded[*record.Supersedes] = true
		}
	}
	linked := make([]Record, len(batch))
	copy(linked, batch)
	for i := range linked {
		if linked[i].Supersedes != nil {
			continue
		}
		earlier, ok := latestCurrent(existing, superseded, linked[i])
		if !ok {
			continue
		}
		id := earlier.RecordID
		linked[i].Supersedes = &id
		superseded[id] = true
	}
	return linked
}

// latestCurrent is the last record in the ledger, in its order, that is still current and is about the same target,
// measure and source kind as record. The ref is not part of the key: it carries the value (a history's fix SHA, or a
// census's Coach hash), so a changed ref is the re-measurement this link exists for.
func latestCurrent(existing []Record, superseded map[string]bool, record Record) (Record, bool) {
	var found Record
	ok := false
	for _, earlier := range existing {
		if superseded[earlier.RecordID] || earlier.Measure != record.Measure ||
			!reflect.DeepEqual(earlier.Target, record.Target) || earlier.Source.Kind != record.Source.Kind {
			continue
		}
		found, ok = earlier, true
	}
	return found, ok
}

// CurrentRecords returns the records no later record supersedes, in ledger order: what a reader counts. The ledger keeps
// every record, so a count over all of them counts a re-measured target once for each measurement.
func CurrentRecords(records []Record) []Record {
	superseded := make(map[string]bool, len(records))
	for _, record := range records {
		if record.Supersedes != nil {
			superseded[*record.Supersedes] = true
		}
	}
	var current []Record
	for _, record := range records {
		if !superseded[record.RecordID] {
			current = append(current, record)
		}
	}
	return current
}
