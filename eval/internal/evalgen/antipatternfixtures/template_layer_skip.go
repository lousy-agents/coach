package antipatternfixtures

import "fmt"

// skippedLayer is #871's skipped layer across the profile's layers: the entry
// handler in the outermost layer reaches the database in the innermost one
// directly, past the required second layer. Its fix routes the call through
// every layer in turn.
func skippedLayer() Template {
	return Template{
		ID: "skipped-layer", Name: "Skipped layer (layer bypass)", Status: "covered",
		observe: observeLayers,
		Bad:     func(layers int) map[string]string { return skipFiles(layers, true) },
		Good:    func(layers int) map[string]string { return skipFiles(layers, false) },
		Padding: layerPadding,
		Expect: func(int) []Finding {
			return []Finding{{
				RuleID: "architecture.layer_bypass", Path: "internal/api/" + layerPackages[0] + "/handle.go",
				Subject: "architecture.layer_bypass:service:example.com/layers/internal/api/" + layerPackages[0] + ".Handle->(*database/sql.DB).Query",
			}}
		},
	}
}

// skipFiles is the layers with a call chain from the entry handler down to the
// query. The handler calls the next layer, or the innermost one when it skips.
func skipFiles(layers int, skip bool) map[string]string {
	roles := layerRoles[layers]
	files := layerBase(layers, "service")
	name := layerPackages[0]
	files[fmt.Sprintf("internal/store/%s/call.go", name)] = querySource(name)
	for i, role := range roles[:len(roles)-1] {
		next := roles[i+1]
		if i == 0 && skip {
			next = roles[len(roles)-1]
		}
		file, source := "call.go", callSource(name, next)
		if i == 0 {
			file, source = "handle.go", handleSource(name, next)
		}
		files[fmt.Sprintf("internal/%s/%s/%s", role, name, file)] = source
	}
	return files
}

func handleSource(name, next string) string {
	return "package " + name + "\n\nimport (\n\t\"database/sql\"\n\t\"net/http\"\n\n\tnext \"example.com/layers/internal/" + next + "/" + name + "\"\n)\n\n" +
		"// Handle is the entry: it passes the request on to the layer that owns the rules.\n" +
		"func Handle(w http.ResponseWriter, r *http.Request) {\n\tvar db *sql.DB\n" +
		"\tif err := next.Call(db, r.URL.Query().Get(\"id\")); err != nil {\n\t\thttp.Error(w, err.Error(), http.StatusInternalServerError)\n\t}\n}\n"
}

func callSource(name, next string) string {
	return "package " + name + "\n\nimport (\n\t\"database/sql\"\n\n\tnext \"example.com/layers/internal/" + next + "/" + name + "\"\n)\n\n" +
		"// Call passes the request on.\nfunc Call(db *sql.DB, id string) error {\n\treturn next.Call(db, id)\n}\n"
}

func querySource(name string) string {
	return "package " + name + "\n\nimport \"database/sql\"\n\n// Call loads a record by id.\nfunc Call(db *sql.DB, id string) error {\n" +
		"\trows, err := db.Query(\"SELECT id FROM records WHERE id = $1\", id)\n\tif err != nil {\n\t\treturn err\n\t}\n\tdefer rows.Close()\n\treturn rows.Err()\n}\n"
}
