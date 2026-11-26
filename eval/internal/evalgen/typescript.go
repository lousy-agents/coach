package evalgen

// tsSpec is everything one TypeScript program is built from.
type tsSpec struct {
	cell   Cell
	name   string
	entity string
	field  string
	shape  tsShape
	layers []string
	route  []hop
}

// modulePath is the file of one package, relative to the program root.
func modulePath(layer, name string) string { return "src/" + layer + "/" + name + ".ts" }

// testPath is the visible test file beside a package's module.
func testPath(layer, name string) string { return "src/" + layer + "/" + name + ".test.ts" }

func (g tsSpec) path(h hop) string { return modulePath(h.layer, h.name) }

// importFrom is the specifier by which one package imports another: each sits in a directory of its layer under src/.
func importFrom(from, to hop) string { return "../" + to.layer + "/" + to.name + ".ts" }

// GenerateTypeScript emits the TypeScript program for a cell and a seed. It is deterministic: the same generator
// version, cell and seed give the same bytes.
func GenerateTypeScript(cell Cell, seed int64) (Program, error) {
	if err := cell.Validate(); err != nil {
		return Program{}, err
	}
	s := newStream(LanguageTypeScript.Version(), cell, seed)
	layers := layerNames[cell.Layers]
	names := shuffled(s, packageNames)[:cell.Packages]
	entity, field := pick(s, entityNames), pick(s, fieldNames[cell.Field])
	g := tsSpec{
		cell: cell, name: pick(s, domainNames), entity: entity, field: lowerFirst(field),
		shape: tsShapeOf(cell.Field, field), layers: layers,
	}
	g.route = route(layers, names, cell.Depth, s)

	tree := map[string][]byte{
		"package.json":                 packageJSON(g.name),
		"tsconfig.json":                []byte(tsConfigJSON),
		".lousy-agents/coach.json":     policyDocument(layers, "src/", string(LanguageTypeScript)),
		"scripts/coach-self-review.sh": selfReviewScript,
	}
	onRoute := map[string]int{}
	for index, h := range g.route {
		onRoute[h.layer+"/"+h.name] = index
	}
	for _, layer := range layers {
		for _, name := range names {
			index, on := onRoute[layer+"/"+name]
			tree[modulePath(layer, name)] = g.packageSource(layer, name, on, index)
			tree[testPath(layer, name)] = g.packageTest(layer, name, on, index)
		}
	}
	program := Program{
		Cell: cell, Seed: seed, Language: LanguageTypeScript, Tree: tree,
		Tests:    map[string][]byte{"tests/feature/feature.test.ts": g.featureTest()},
		Solution: map[string][]byte{g.path(g.route[0]): g.entrySource(true)},
	}
	program.ID = TreeID(tree)
	program.Manifest = buildTypeScriptManifest(g, program)
	return program, nil
}

func (g tsSpec) packageSource(layer, name string, onRoute bool, index int) []byte {
	switch {
	case !onRoute:
		return extraModule(layer, name)
	case index == 0:
		return g.entrySource(false)
	case index == g.cell.Depth:
		return g.entityModule()
	}
	return g.passthroughModule(index)
}
