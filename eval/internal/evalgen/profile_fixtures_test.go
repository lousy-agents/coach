package evalgen_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func acceptAll([]byte) error { return nil }

// profileJSON is a profile in the committed schema's shape with the given
// dimensions; roles carry packages per layer.
func profileJSON(t *testing.T, layers, depth map[string]int, entry map[string]int, roles map[string]map[string]int) []byte {
	t.Helper()
	roleObjects := map[string]any{}
	for role, packages := range roles {
		roleObjects[role] = map[string]any{"packages": packages}
	}
	raw, err := json.Marshal(map[string]any{
		"schema_version": "1", "identity": strings.Repeat("ab", 32), "label": "stand-in",
		"sha": strings.Repeat("0", 40), "licence": "MIT", "extractor_version": "0.2.0",
		"roles": roleObjects,
		"dimensions": map[string]any{
			"layers": layers, "entry": entry, "depth": depth, "trap": nil, "field": nil, "direction": nil,
		},
		"shapes": map[string]any{
			"nesting": map[string]int{}, "cognitive": map[string]int{}, "branch_density": map[string]int{},
			"constructor": map[string]int{"assigns_only": 0, "does_work": 0},
		},
		"not_covered": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func readProfile(t *testing.T, raw []byte) evalgen.Profile {
	t.Helper()
	profile, err := evalgen.ReadProfile(raw, acceptAll)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func skewedProfile(t *testing.T) evalgen.Profile {
	return readProfile(t, profileJSON(t, map[string]int{"5": 40}, map[string]int{"2": 40},
		map[string]int{"function": 0, "method": 0, "http_handler": 0}, nil))
}

func equalCells(a, b []evalgen.Cell) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func closeTo(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= 1e-9*(a+b)
}

func itoa(n int) string { return strings.TrimSpace(string(rune('0' + n))) }

func distinct(cells []evalgen.Cell) bool {
	seen := map[int]bool{}
	for _, cell := range cells {
		if seen[cell.Index()] {
			return false
		}
		seen[cell.Index()] = true
	}
	return true
}

func countWhere(cells []evalgen.Cell, holds func(evalgen.Cell) bool) int {
	count := 0
	for _, cell := range cells {
		if holds(cell) {
			count++
		}
	}
	return count
}
