package evaldetect_test

import (
	. "github.com/onsi/ginkgo/v2"
)

const bypassRule = "architecture.layer_bypass"

// bypassPolicy layers store under usecases under api, with usecases the
// required layer. apiToStore adds a forbidden api→store pair when set.
func bypassPolicy(requiredLayer string, apiToStore bool) string {
	forbidden := `{"from":"store","to":"usecases"},{"from":"store","to":"api"},{"from":"usecases","to":"api"}`
	if apiToStore {
		forbidden += `,{"from":"api","to":"store"}`
	}
	required := ""
	if requiredLayer != "" {
		required = `,"required_layer":"` + requiredLayer + `"`
	}
	return `{"schema_version":"1","roots":["."],"layers":[` +
		`{"name":"store","prefixes":["pkg/store"]},` +
		`{"name":"usecases","prefixes":["pkg/usecases"]},` +
		`{"name":"api","prefixes":["pkg/api"]}],` +
		`"forbidden_imports":[` + forbidden + `]` + required + `}`
}

// bypassTree is a compliant program: api.Handle reaches store.Save only
// through usecases.Register. extra adds or replaces files.
func bypassTree(policy string, extra map[string]string) string {
	files := map[string]string{
		"go.mod":                    "module example.com/fx\n\ngo 1.26.0\n",
		".lousy-agents/coach.json":  policy,
		"pkg/store/store.go":        "package store\n\ntype Record struct{ Name string }\n\nfunc Save(s string) {}\n",
		"pkg/usecases/usecases.go":  "package usecases\n\nimport \"example.com/fx/pkg/store\"\n\nfunc Register(s string) { store.Save(s) }\n",
		"pkg/api/api.go":            "package api\n\nimport \"example.com/fx/pkg/usecases\"\n\nfunc Handle(s string) { usecases.Register(s) }\n",
		"pkg/api/api_test.go":       "package api\n\nimport \"example.com/fx/pkg/store\"\n\nfunc testOnly() { store.Save(\"t\") }\n",
		"pkg/store/testdata/x/x.go": "package x\n\nfunc broken( {\n",
	}
	for path, content := range extra {
		files[path] = content
	}
	return writeTree(GinkgoT().TempDir(), files)
}

const directBypass = "package api\n\nimport \"example.com/fx/pkg/store\"\n\nfunc Direct(s string) { store.Save(s) }\n"
