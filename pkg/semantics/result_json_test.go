package semantics

import (
	"encoding/json"
	"reflect"
	"testing"
)

// AC-4.3: ParseStatus must serialize as exactly "ok" or "syntax_errors";
// no other values exist in v1.
func TestParseStatus_SerializesAsOkOrSyntaxErrors(t *testing.T) {
	tests := []parseStatusJSONCase{
		{name: "ok status", status: ParseStatus("ok"), want: `"ok"`},
		{name: "syntax_errors status", status: ParseStatus("syntax_errors"), want: `"syntax_errors"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectParseStatusJSON(t, tt)
		})
	}
}

type parseStatusJSONCase struct {
	name   string
	status ParseStatus
	want   string
}

func expectParseStatusJSON(t *testing.T, tt parseStatusJSONCase) {
	raw, err := json.Marshal(tt.status)
	if err != nil {
		t.Fatalf("AC-4.3: marshaling ParseStatus %q must not fail: %v", tt.status, err)
	}
	if string(raw) != tt.want {
		t.Errorf("AC-4.3: ParseStatus %q marshaled: got %s, want %s", tt.status, string(raw), tt.want)
	}
}

// AC-4.2: Location must serialize exactly start_byte, end_byte, start_row,
// start_col, end_row, end_col, preserving the 0-based values Tree-sitter
// reports (no offset applied during marshaling).
func TestLocation_JSONHasZeroBasedByteAndRowColFields(t *testing.T) {
	loc := Location{
		StartByte: 0,
		EndByte:   7,
		StartRow:  0,
		StartCol:  0,
		EndRow:    0,
		EndCol:    7,
	}

	raw, err := json.Marshal(loc)
	if err != nil {
		t.Fatalf("AC-4.2: marshaling a Location must not fail: %v", err)
	}

	want := `{"start_byte":0,"end_byte":7,"start_row":0,"start_col":0,"end_row":0,"end_col":7}`
	if string(raw) != want {
		t.Errorf("AC-4.2: Location JSON fields/order/0-based values: got %s, want %s", string(raw), want)
	}
}

// AC-3.7: Finding's grammar-node facts (Kind, Name, Location) are required
// and must stay first, in this order; the remaining fields are optional
// coaching metadata (Confidence, Evidence, Recommendation, SuggestedSkill)
// used by findings like "mutates_input" and omitted via omitempty for
// findings that don't set them. Checked structurally via reflection rather
// than by code review, so a future field addition fails this test loudly.
func TestFinding_StructCarriesOnlyDataFields(t *testing.T) {
	typ := reflect.TypeOf(Finding{})

	wantFields := []string{"Kind", "Name", "Location", "Confidence", "Evidence", "Recommendation", "SuggestedSkill"}
	if typ.NumField() != len(wantFields) {
		t.Fatalf("Finding field count: got %d fields, want exactly %d (%v)", typ.NumField(), len(wantFields), wantFields)
	}
	for i, want := range wantFields {
		if got := typ.Field(i).Name; got != want {
			t.Errorf("Finding field %d: got %q, want %q", i, got, want)
		}
	}
}
