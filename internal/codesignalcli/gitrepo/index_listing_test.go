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
		"a mode with a digit that is not octal":                  "H 100648 " + listingBlob + " 0\tpol.json",
		"a mode with a digit that is not octal, in the middle":   "H 108644 " + listingBlob + " 0\tpol.json",
		"a mode of five digits":                                  "H 10064 " + listingBlob + " 0\tpol.json",
		"a mode of seven digits":                                 "H 1006440 " + listingBlob + " 0\tpol.json",
		"an object id in upper case":                             "H 100644 " + "ABCDEFABCDEFABCDEFABCDEFABCDEFABCDEFABCD" + " 0\tpol.json",
		"an object id with a letter past f":                      "H 100644 " + "g123456789012345678901234567890123456789" + " 0\tpol.json",
		"an object id of 39 digits":                              "H 100644 " + listingBlob[1:] + " 0\tpol.json",
		"an object id of 41 digits":                              "H 100644 " + listingBlob + "0 0\tpol.json",
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

func TestParseIndexRecordReadsTheObjectIdsOfBothHashes(t *testing.T) {
	sha256 := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	for _, id := range []string{listingBlob, sha256} {
		record := "H 100755 " + id + " 0\tpol.json"

		entry, path, err := parseIndexRecord(record)

		if err != nil || entry.SHA != id || entry.Mode != "100755" || path != "pol.json" {
			t.Errorf("parseIndexRecord(%q) = %+v, %q, %v; want the entry read", record, entry, path, err)
		}
	}
}
