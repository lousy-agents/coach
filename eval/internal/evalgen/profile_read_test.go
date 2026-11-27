package evalgen_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func TestReadProfileRefusesWhatItCannotRead(t *testing.T) {
	raw := profileJSON(t, map[string]int{"3": 1}, map[string]int{}, map[string]int{"function": 1, "method": 0, "http_handler": 0}, nil)
	cases := map[string]struct {
		raw      []byte
		validate func([]byte) error
		reason   string
	}{
		"the validator rejects it": {raw, func([]byte) error { return errors.New("closed schema") }, "closed schema"},
		"another extractor":        {[]byte(strings.Replace(string(raw), `"0.2.0"`, `"0.1.0"`, 1)), acceptAll, `extractor_version "0.1.0"`},
		"another schema version":   {[]byte(strings.Replace(string(raw), `"schema_version":"1"`, `"schema_version":"2"`, 1)), acceptAll, `schema_version "2"`},
	}
	for name, c := range cases {
		if _, err := evalgen.ReadProfile(c.raw, c.validate); err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Fatalf("%s: got %v, want an error naming %s", name, err, c.reason)
		}
	}
}

// TestKeyWithoutAProfileIsUnchanged pins the cache key of a fixed cell, so a
// run without a profile reuses every cache entry it had (AC-3.5).
func TestKeyWithoutAProfileIsUnchanged(t *testing.T) {
	cell, _ := evalgen.CellFromIndex(1000)
	const before = "9c743267c7a0c58fbf88575a3e8be6c24ea0eadacf5dc6a766259d64125a6c71"
	if got := evalgen.Key(evalgen.LanguageGo, cell, 7, "tuning", ""); got != before {
		t.Fatalf("the key without a profile is %s, it was %s", got, before)
	}
	if evalgen.Key(evalgen.LanguageGo, cell, 7, "tuning", "abc") == before {
		t.Fatal("a profile run shares the key of a run without one")
	}
}
