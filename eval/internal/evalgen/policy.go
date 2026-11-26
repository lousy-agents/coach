package evalgen

import (
	_ "embed"
	"encoding/json"
)

// selfReviewScript is the block in docs/cli-codesignal.md "### The check", byte for byte; a spec compares them.
//
//go:embed coach-self-review.sh
var selfReviewScript []byte

type policyLayer struct {
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes"`
}

type policyEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type policy struct {
	SchemaVersion    string        `json:"schema_version"`
	Roots            []string      `json:"roots"`
	Layers           []policyLayer `json:"layers"`
	ForbiddenImports []policyEdge  `json:"forbidden_imports"`
	RequiredLayer    string        `json:"required_layer"`
}

// policyJSON forbids the one pair that has no reverse path (the entry layer reaching the data layer) and
// requires the second layer on every route from the entry layer to the data layer.
func policyJSON(layers []string) []byte {
	declared := make([]policyLayer, 0, len(layers))
	for _, name := range layers {
		declared = append(declared, policyLayer{Name: name, Prefixes: []string{"internal/" + name}})
	}
	data, _ := json.MarshalIndent(policy{
		SchemaVersion:    "1",
		Roots:            []string{"."},
		Layers:           declared,
		ForbiddenImports: []policyEdge{{From: layers[0], To: layers[len(layers)-1]}},
		RequiredLayer:    layers[1],
	}, "", "  ")
	return append(data, '\n')
}
