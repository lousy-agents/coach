package evalgen

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

const (
	// GeneratorVersion changes whenever the emitted trees can change for the same cell and seed.
	GeneratorVersion = "go-1"
	// GoDirective matches eval/go.mod, with no toolchain line, so the host's Go always satisfies it.
	GoDirective = "1.26.0"
)

// spec is everything one program is built from.
type spec struct {
	cell   Cell
	module string
	entity string
	field  string
	shape  fieldShape
	layers []string
	names  []string
	route  []hop
}

// Program is one generated program: the tree an agent sees, the hidden FAIL_TO_PASS tests, the clean oracle
// (files that overlay the tree), and the manifest.
type Program struct {
	ID       string
	Language Language
	Cell     Cell
	Seed     int64
	Tree     map[string][]byte
	Tests    map[string][]byte
	Solution map[string][]byte
	Manifest Manifest
}

// Generate emits the program for a cell and a seed. It is deterministic: the same generator version, cell and
// seed give the same bytes.
func Generate(cell Cell, seed int64) (Program, error) {
	if err := cell.Validate(); err != nil {
		return Program{}, err
	}
	s := newStream(GeneratorVersion, cell, seed)
	layers := layerNames[cell.Layers]
	names := shuffled(s, packageNames)[:cell.Packages]
	entity, field := pick(s, entityNames), pick(s, fieldNames[cell.Field])
	g := spec{
		cell: cell, module: "example.com/" + pick(s, domainNames), entity: entity, field: field,
		shape: shapeOf(cell.Field, field), layers: layers, names: names,
	}
	g.route = route(layers, names, cell.Depth, s)

	tree := map[string][]byte{
		"go.mod":                       []byte("module " + g.module + "\n\ngo " + GoDirective + "\n"),
		".lousy-agents/coach.json":     policyJSON(layers),
		"scripts/coach-self-review.sh": selfReviewScript,
	}
	onRoute := map[string]int{}
	for index, h := range g.route {
		onRoute[h.dir] = index
	}
	for _, layer := range layers {
		for _, name := range names {
			dir := "internal/" + layer + "/" + name
			index, on := onRoute[dir]
			tree[dir+"/"+name+".go"] = packageSource(g, layer, name, on, index)
			tree[dir+"/"+name+"_test.go"] = packageTest(g, layer, name, on, index)
		}
	}
	program := Program{
		Cell: cell, Seed: seed, Language: LanguageGo, Tree: tree,
		Tests:    map[string][]byte{"tests/feature/feature_test.go": featureTest(g)},
		Solution: map[string][]byte{g.route[0].dir + "/" + g.route[0].name + ".go": entrySource(g, true)},
	}
	program.ID = TreeID(tree)
	program.Manifest = buildManifest(g, program)
	return program, nil
}

func packageSource(g spec, layer, name string, onRoute bool, index int) []byte {
	switch {
	case !onRoute:
		return extraPackage(name, layer)
	case index == 0:
		return entrySource(g, false)
	case index == g.cell.Depth:
		return entityPackage(g)
	}
	return passthroughPackage(g, index)
}

// TreeID is the hash of a tree: sha256 over the sorted "path NUL sha256(content) LF" records.
func TreeID(tree map[string][]byte) string {
	paths := make([]string, 0, len(tree))
	for path := range tree {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		sum := sha256.Sum256(tree[path])
		fmt.Fprintf(hash, "%s\x00%s\n", path, hex.EncodeToString(sum[:]))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// OracleID hashes the hidden tests and the clean oracle together.
func OracleID(p Program) string {
	merged := map[string][]byte{}
	for path, content := range p.Tests {
		merged["tests/"+path] = content
	}
	for path, content := range p.Solution {
		merged["solution/"+path] = content
	}
	return TreeID(merged)
}
