package codesignal

import (
	"encoding/json"
	"testing"
)

func TestParseSeverityFloor(t *testing.T) {
	accepted := map[string]bool{
		"high": true, "medium": true, "advisory": true, "low": true,
		"": false, "HIGH": false, "urgent": false, "critical": false, " high": false,
	}
	for value, wantAccepted := range accepted {
		if got, ok := ParseSeverityFloor(value); ok != wantAccepted || (ok && string(got) != value) {
			t.Fatalf("ParseSeverityFloor(%q) = %q, %t, want accepted=%t", value, got, ok, wantAccepted)
		}
	}
}

func signalsWithheldWire(t *testing.T, report *Report) (string, bool) {
	t.Helper()
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	raw, present := document["signals_withheld"]
	return string(raw), present
}

func assertSignalsWithheldWire(t *testing.T, schemaVersion string, opts NarrowOptions, want string) {
	t.Helper()
	report := reportWithSeverities(schemaVersion, "high", "high", "low")
	if _, present := signalsWithheldWire(t, report); present {
		t.Fatalf("schema %s: an unnarrowed report must not emit signals_withheld", schemaVersion)
	}
	view := mustNarrow(t, report, opts)
	if got, _ := signalsWithheldWire(t, &view); got != want {
		t.Fatalf("signals_withheld = %s, want %s", got, want)
	}
}

var withheldWireShapes = map[string]struct {
	opts NarrowOptions
	want string
}{
	"floor only":     {NarrowOptions{MinSeverity: "high"}, `{"min_severity":"high","below_min_severity":1}`},
	"top only":       {NarrowOptions{Top: 1}, `{"top":1,"beyond_top":2}`},
	"top above size": {NarrowOptions{Top: 9}, `{"top":9,"beyond_top":0}`},
	"floor and top":  {NarrowOptions{MinSeverity: "high", Top: 1}, `{"min_severity":"high","below_min_severity":1,"top":1,"beyond_top":1}`},
	"floor, zero":    {NarrowOptions{MinSeverity: "low"}, `{"min_severity":"low","below_min_severity":0}`},
}

func assertWithheldWireShapesForSchema(t *testing.T, schemaVersion string) {
	t.Helper()
	for name, shape := range withheldWireShapes {
		t.Run(name, func(t *testing.T) {
			assertSignalsWithheldWire(t, schemaVersion, shape.opts, shape.want)
		})
	}
}

func TestSignalsWithheldWireShapePerNarrowing(t *testing.T) {
	for _, schemaVersion := range []string{"1", "2"} {
		t.Run("schema "+schemaVersion, func(t *testing.T) {
			assertWithheldWireShapesForSchema(t, schemaVersion)
		})
	}
}
