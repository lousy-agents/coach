package gitrepo

import "testing"

// The listing is read by what each field holds, so a field a different listing
// could print is refused rather than read as the nearest thing.
func TestParseIndexRecordRefusesATagOrStageAnIndexEntryNeverHas(t *testing.T) {
	const blob = "0123456789012345678901234567890123456789"
	cases := []struct {
		name   string
		record string
	}{
		{"the tag for a file git reports as changed", "C 100644 " + blob + " 0\tpol.json"},
		{"the tag for a removed file", "R 100644 " + blob + " 0\tpol.json"},
		{"the tag for a killed file", "K 100644 " + blob + " 0\tpol.json"},
		{"the tag for an unmerged file in the -u listing", "U 100644 " + blob + " 0\tpol.json"},
		{"two stages run together, whose digits are each a stage", "H 100644 " + blob + " 12\tpol.json"},
		{"an empty stage", "H 100644 " + blob + " \tpol.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if entry, path, err := parseIndexRecord(c.record); err == nil {
				t.Errorf("parseIndexRecord(%q) = %+v, %q, nil; want an error", c.record, entry, path)
			}
		})
	}
}

func TestParseIndexRecordReadsEveryTagAnIndexEntryHas(t *testing.T) {
	const blob = "0123456789012345678901234567890123456789"
	for _, tag := range []string{"H", "S", "M"} {
		for _, stage := range []string{"0", "1", "2", "3"} {
			record := tag + " 100644 " + blob + " " + stage + "\tpol.json"

			entry, path, err := parseIndexRecord(record)

			if err != nil || path != "pol.json" || entry.Stage != stage || entry.SkipWorktree != (tag == "S") {
				t.Errorf("parseIndexRecord(%q) = %+v, %q, %v", record, entry, path, err)
			}
		}
	}
}
