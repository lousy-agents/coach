package gitrepo

import "testing"

const listingBlob = "0123456789012345678901234567890123456789"

// The listing is read by what each field holds, so a field a different listing
// could print is refused rather than read as the nearest thing.
func TestParseIndexRecordRefusesATagOrStageAnIndexEntryNeverHas(t *testing.T) {
	cases := map[string]string{
		"the tag for a file git reports as changed":              "C 100644 " + listingBlob + " 0\tpol.json",
		"the tag for a removed file":                             "R 100644 " + listingBlob + " 0\tpol.json",
		"the tag for a killed file":                              "K 100644 " + listingBlob + " 0\tpol.json",
		"the tag for an unmerged file in the -u listing":         "U 100644 " + listingBlob + " 0\tpol.json",
		"two stages run together, whose digits are each a stage": "H 100644 " + listingBlob + " 12\tpol.json",
		"an empty stage":                                         "H 100644 " + listingBlob + " \tpol.json",
	}
	for name, record := range cases {
		t.Run(name, func(t *testing.T) { requireRecordRefused(t, record) })
	}
}

func requireRecordRefused(t *testing.T, record string) {
	t.Helper()
	entry, path, err := parseIndexRecord(record)
	if err == nil {
		t.Errorf("parseIndexRecord(%q) = %+v, %q, nil; want an error", record, entry, path)
	}
}

func TestParseIndexRecordReadsEveryTagAnIndexEntryHas(t *testing.T) {
	for _, tag := range []string{"H", "S", "M"} {
		for _, stage := range []string{"0", "1", "2", "3"} {
			requireRecordRead(t, tag, stage)
		}
	}
}

func requireRecordRead(t *testing.T, tag, stage string) {
	t.Helper()
	record := tag + " 100644 " + listingBlob + " " + stage + "\tpol.json"

	entry, path, err := parseIndexRecord(record)

	if err != nil || path != "pol.json" || entry.Stage != stage || entry.SkipWorktree != (tag == "S") {
		t.Errorf("parseIndexRecord(%q) = %+v, %q, %v", record, entry, path, err)
	}
}
