package projectconfig

import (
	"fmt"
	"testing"
)

// spellingIn lists a directory entryListingBatch names at a time. A name that lies beyond the first batch
// must still be found, or a large directory hides the spelling the check exists to name.
func TestTheSpellingOfANameIsFoundBeyondTheFirstBatchOfEntries(t *testing.T) {
	cases := []struct {
		name string
		look string
		want string
	}{
		{"a case variant in the second batch", "zz-p.json", "zz-P.json"},
		{"the exact name in the second batch, with a case variant in the first", "a0000.json", "a0000.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertSpellingAcrossBatches(t, c.look, c.want)
		})
	}
}

func assertSpellingAcrossBatches(t *testing.T, name, want string) {
	t.Helper()
	paths := make([]string, 0, entryListingBatch+2)
	for i := 0; i <= entryListingBatch; i++ {
		paths = append(paths, fmt.Sprintf("cfg/a%04d.json", i))
	}
	paths = append(paths, "cfg/zz-P.json", "cfg/A0000.JSON")

	got, err := spellingIn(foldingDiskHolding(paths...), "cfg", name)

	if err != nil || got != want {
		t.Errorf("spellingIn(%q) = %q, %v; want %q", name, got, err, want)
	}
}

func TestTheExactNameInALaterBatchWinsOverACaseVariantInAnEarlierOne(t *testing.T) {
	paths := []string{"cfg/P.json"}
	for i := 0; i < entryListingBatch; i++ {
		paths = append(paths, fmt.Sprintf("cfg/b%04d.json", i))
	}
	paths = append(paths, "cfg/p.json")

	got, err := spellingIn(foldingDiskHolding(paths...), "cfg", "p.json")

	if err != nil || got != "p.json" {
		t.Errorf("spellingIn = %q, %v; want the exact name p.json, found in the second batch, over P.json from the first", got, err)
	}
}
