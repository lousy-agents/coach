package semantics

import (
	"reflect"
	"testing"
)

// Constructor validation (supports AC-1.5 at the boundary): NewAnalyzer must
// reject any Languages entry that isn't a recognized Language, since
// AnalyzeBytes has no other opportunity to validate the configured set up
// front.
func TestNewAnalyzer_RejectsUnknownLanguageNames(t *testing.T) {
	_, err := NewAnalyzer(AnalyzerOptions{Languages: []Language{"python"}})

	if err == nil {
		t.Fatalf("NewAnalyzer with Languages containing an unrecognized language %q: got nil error, want non-nil", "python")
	}
}

// Constructor validation: NewAnalyzer must reject a negative MaxFileBytes,
// since a negative size limit is nonsensical (per the frozen API doc: "0 =
// default 2 MiB; negative = NewAnalyzer returns an error").
func TestNewAnalyzer_RejectsNegativeMaxFileBytes(t *testing.T) {
	_, err := NewAnalyzer(AnalyzerOptions{MaxFileBytes: -1})

	if err == nil {
		t.Fatalf("NewAnalyzer with MaxFileBytes = -1: got nil error, want non-nil")
	}
}

// AC-6.3: Analyzer must hold no C-backed resources between calls (Parser,
// Tree, Query, QueryCursor are all created fresh inside AnalyzeBytes), so it
// must expose no Close method for callers to forget to call.
func TestAnalyzer_HasNoExportedCloseMethod(t *testing.T) {
	_, ok := reflect.TypeOf(&Analyzer{}).MethodByName("Close")

	if ok {
		t.Errorf("AC-6.3: (*Analyzer).Close must not exist, want ok == false, got ok == true")
	}
}
