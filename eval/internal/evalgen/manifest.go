package evalgen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Manifest is the program and template part of manifest.json (design §11). Rows belong to the operators.
type Manifest struct {
	SchemaVersion string         `json:"schema_version"`
	ID            string         `json:"id"`
	Kind          string         `json:"kind"`
	Language      []string       `json:"language"`
	Program       ProgramInfo    `json:"program"`
	Operators     []any          `json:"operators"`
	Validation    map[string]any `json:"validation"`
	Rows          []any          `json:"rows"`
	Template      TemplateInfo   `json:"template"`
}

type ProgramInfo struct {
	ProgramID        string `json:"program_id"`
	GeneratorVersion string `json:"generator_version"`
	Cell             Cell   `json:"cell"`
	SeedHash         string `json:"seed_hash"`
	Panel            string `json:"panel"`
	Replacements     int    `json:"replacements"`
	Profile          string `json:"profile,omitempty"`
}

type TemplateInfo struct {
	TemplateID          string        `json:"template_id"`
	APISurface          []APISymbol   `json:"api_surface"`
	VisibleTests        []VisibleTest `json:"visible_tests"`
	DependencyWhitelist []string      `json:"dependency_whitelist"`
	OracleRef           string        `json:"oracle_ref"`
	FollowOn            *FollowOnInfo `json:"follow_on,omitempty"`
	LOCBand             [2]float64    `json:"loc_band"`
}

// FollowOnInfo names the follow-on slot: the entry must return Field beside FirstField. The request prose is
// rendered from these by the harness.
type FollowOnInfo struct {
	Field      string `json:"field"`
	FirstField string `json:"first_field"`
	Entry      string `json:"entry"`
}

type APISymbol struct {
	Symbol string `json:"symbol"`
	Entry  string `json:"entry"`
}

type VisibleTest struct {
	Path    string   `json:"path"`
	SHA256  string   `json:"sha256"`
	TestIDs []string `json:"test_ids"`
}

var testFunction = regexp.MustCompile(`(?m)^func (Test\w+)\(`)

func buildManifest(g spec, p Program) Manifest {
	cellJSON, _ := json.Marshal(g.cell)
	templateSum := sha256.Sum256([]byte(GeneratorVersion + "|" + string(cellJSON)))
	seedSum := sha256.Sum256([]byte(fmt.Sprint(p.Seed)))
	entry := g.route[0].dir + "." + g.entryName()
	surface := []APISymbol{{Symbol: g.module + "/" + entry, Entry: entry}}
	function := "Load"
	if g.cell.Direction == "write" {
		function = "Update"
	}
	for _, h := range g.route[1:] {
		surface = append(surface, APISymbol{Symbol: g.module + "/" + h.dir + "." + function, Entry: entry})
	}
	return Manifest{
		SchemaVersion: "1", ID: p.ID, Kind: "feature_template_instance", Language: []string{"go"},
		Program: ProgramInfo{
			ProgramID: p.ID, GeneratorVersion: GeneratorVersion, Cell: g.cell,
			SeedHash: hex.EncodeToString(seedSum[:]), Panel: "tuning",
		},
		Operators:  []any{},
		Validation: map[string]any{},
		Rows:       []any{},
		Template: TemplateInfo{
			TemplateID: hex.EncodeToString(templateSum[:]), APISurface: surface, VisibleTests: visibleTests(p.Tree),
			DependencyWhitelist: []string{}, OracleRef: OracleID(p), FollowOn: &FollowOnInfo{Field: g.second, FirstField: g.field, Entry: entry}, LOCBand: [2]float64{0.7, 1.5},
		},
	}
}

func visibleTests(tree map[string][]byte) []VisibleTest {
	tests := []VisibleTest{}
	for path, content := range tree {
		if !strings.HasSuffix(path, "_test.go") {
			continue
		}
		sum := sha256.Sum256(content)
		ids := []string{}
		for _, match := range testFunction.FindAllSubmatch(content, -1) {
			ids = append(ids, string(match[1]))
		}
		tests = append(tests, VisibleTest{Path: path, SHA256: hex.EncodeToString(sum[:]), TestIDs: ids})
	}
	sort.Slice(tests, func(i, j int) bool { return tests[i].Path < tests[j].Path })
	return tests
}
