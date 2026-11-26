package evalgrade

import (
	"fmt"
	"strings"
	"testing"
)

const genuine = `{"nonce":"n","id":"a.test.ts::one","status":"pass"}` + "\n"

func TestALineLongerThanAnyRecordCountsAsMalformedEvenWhenItsStartParses(t *testing.T) {
	padded := `{"nonce":"n","id":"a.test.ts::padded","status":"pass"}` + strings.Repeat(" ", 2<<20) + "\n"

	run := readRecords(strings.NewReader(padded + genuine))

	if len(run.Records) != 1 || run.Records[0].ID != "a.test.ts::one" || run.Malformed != 1 {
		t.Fatalf("records=%v malformed=%d: the padded line must count as malformed and the line after it still be read", run.Records, run.Malformed)
	}
}

func TestAFinalLineWithoutItsNewlineCountsAsMalformedWhenItIsCutShort(t *testing.T) {
	run := readRecords(strings.NewReader(genuine + `{"nonce":`))

	if len(run.Records) != 1 || run.Malformed != 1 {
		t.Fatalf("records=%v malformed=%d, want the one record and one malformed line", run.Records, run.Malformed)
	}
}

func TestARunThatWritesFarMoreRecordsThanAnyProgramHoldsKeepsOnlyABoundedNumberAndCountsTheRest(t *testing.T) {
	const written = 250_000
	var lines strings.Builder
	for i := 0; i < written; i++ {
		fmt.Fprintf(&lines, `{"nonce":"n","id":"a.test.ts::t%d","status":"pass"}`+"\n", i)
	}

	run := readRecords(strings.NewReader(lines.String()))

	if len(run.Records) > 200_000 {
		t.Fatalf("%d records kept: a test writing to the channel can grow memory without limit", len(run.Records))
	}
	if len(run.Records)+run.Malformed != written {
		t.Fatalf("kept %d and counted %d past the cap, want all %d accounted for", len(run.Records), run.Malformed, written)
	}
}
