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

var tsTestName = regexp.MustCompile(`(?m)^test\("([^"]+)"`)

func buildTypeScriptManifest(g tsSpec, p Program) Manifest {
	cellJSON, _ := json.Marshal(g.cell)
	templateSum := sha256.Sum256([]byte(LanguageTypeScript.Version() + "|" + string(cellJSON)))
	seedSum := sha256.Sum256([]byte(fmt.Sprint(p.Seed)))
	entry := g.path(g.route[0]) + "#" + g.entrySymbol()
	surface := []APISymbol{{Symbol: entry, Entry: entry}}
	function := "load"
	if g.cell.Direction == "write" {
		function = "update"
	}
	for _, h := range g.route[1:] {
		surface = append(surface, APISymbol{Symbol: g.path(h) + "#" + function, Entry: entry})
	}
	return Manifest{
		SchemaVersion: "1", ID: p.ID, Kind: "feature_template_instance", Language: []string{string(LanguageTypeScript)},
		Program: ProgramInfo{
			ProgramID: p.ID, GeneratorVersion: LanguageTypeScript.Version(), Cell: g.cell,
			SeedHash: hex.EncodeToString(seedSum[:]), Panel: "tuning",
		},
		Operators:  []any{},
		Validation: map[string]any{},
		Rows:       []any{},
		Template: TemplateInfo{
			TemplateID: hex.EncodeToString(templateSum[:]), APISurface: surface, VisibleTests: visibleTypeScriptTests(p.Tree),
			DependencyWhitelist: []string{}, OracleRef: OracleID(p), LOCBand: [2]float64{0.7, 1.5},
		},
	}
}

func visibleTypeScriptTests(tree map[string][]byte) []VisibleTest {
	tests := []VisibleTest{}
	for path, content := range tree {
		if !strings.HasSuffix(path, ".test.ts") {
			continue
		}
		sum := sha256.Sum256(content)
		ids := []string{}
		for _, match := range tsTestName.FindAllSubmatch(content, -1) {
			ids = append(ids, string(match[1]))
		}
		tests = append(tests, VisibleTest{Path: path, SHA256: hex.EncodeToString(sum[:]), TestIDs: ids})
	}
	sort.Slice(tests, func(i, j int) bool { return tests[i].Path < tests[j].Path })
	return tests
}
