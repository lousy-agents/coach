package antipatternfixtures

import (
	"encoding/json"
	"fmt"
)

const layerPolicyPath = ".lousy-agents/coach.json"

// layerPackages are the packages a layer holds, the first of which every
// layer fixture builds on.
var layerPackages = [...]string{"orders", "billing", "audit"}

type policyLayer struct {
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes"`
}

type policyPair struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type layerPolicy struct {
	SchemaVersion    string        `json:"schema_version"`
	Roots            []string      `json:"roots"`
	Layers           []policyLayer `json:"layers"`
	ForbiddenImports []policyPair  `json:"forbidden_imports"`
	RequiredLayer    string        `json:"required_layer,omitempty"`
}

// policyFor orders the layers by forbidden pairs: every inner layer may not
// import any outer one, so each pair of layers is forbidden one way.
func policyFor(layers int, required string) string {
	roles := layerRoles[layers]
	policy := layerPolicy{SchemaVersion: "1", Roots: []string{"."}, RequiredLayer: required}
	for outer, role := range roles {
		policy.Layers = append(policy.Layers, policyLayer{Name: role, Prefixes: []string{"internal/" + role}})
		for _, inner := range roles[outer+1:] {
			policy.ForbiddenImports = append(policy.ForbiddenImports, policyPair{From: inner, To: role})
		}
	}
	raw, _ := json.Marshal(policy)
	return string(raw)
}

// describeStub is a package that names itself and imports nothing.
func describeStub(role, name string) string {
	return fmt.Sprintf("package %s\n\n// Describe names the package.\nfunc Describe() string {\n\treturn \"%s/%s\"\n}\n", name, role, name)
}

func stubPath(role, name string) string {
	return fmt.Sprintf("internal/%s/%s/%s.go", role, name, name)
}

// layerBase is what both layer fixtures share: the module, the policy and a
// stub for the first package of every layer.
func layerBase(layers int, required string) map[string]string {
	files := map[string]string{"go.mod": goMod("layers"), layerPolicyPath: policyFor(layers, required)}
	for _, role := range layerRoles[layers] {
		files[stubPath(role, layerPackages[0])] = describeStub(role, layerPackages[0])
	}
	return files
}

// layerPadding holds the packages beyond the first in every layer, as stubs.
func layerPadding(layers, packages int) map[string]string {
	files := map[string]string{}
	for _, role := range layerRoles[layers] {
		for _, name := range layerPackages[1:packages] {
			files[stubPath(role, name)] = describeStub(role, name)
		}
	}
	return files
}
