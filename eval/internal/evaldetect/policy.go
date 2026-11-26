package evaldetect

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Policy is the layer model a detector reads: the same .lousy-agents/coach.json
// the self-review hook reads.
type Policy struct {
	layers    []policyLayer
	forbidden map[[2]string]bool
}

type policyLayer struct {
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes"`
}

type policyFile struct {
	Layers           []policyLayer `json:"layers"`
	ForbiddenImports []struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"forbidden_imports"`
}

func LoadPolicy(path string) (Policy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, fmt.Errorf("read layer policy: %w", err)
	}
	var file policyFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return Policy{}, fmt.Errorf("parse layer policy %s: %w", path, err)
	}
	policy := Policy{layers: file.Layers, forbidden: map[[2]string]bool{}}
	for _, pair := range file.ForbiddenImports {
		policy.forbidden[[2]string{pair.From, pair.To}] = true
	}
	return policy, nil
}

// layerOf returns the layer whose longest prefix contains dir.
func (p Policy) layerOf(dir string) (string, bool) {
	name, longest := "", -1
	for _, layer := range p.layers {
		if length := longestPrefix(layer.Prefixes, dir); length > longest {
			name, longest = layer.Name, length
		}
	}
	return name, longest >= 0
}

// longestPrefix returns the length of the longest prefix containing dir, or
// -1 when none does.
func longestPrefix(prefixes []string, dir string) int {
	longest := -1
	for _, prefix := range prefixes {
		if prefixContains(prefix, dir) {
			longest = max(longest, len(prefix))
		}
	}
	return longest
}

func prefixContains(prefix, dir string) bool {
	return dir == prefix || strings.HasPrefix(dir, prefix+"/")
}

// forbids reports whether an import from fromDir to toDir crosses a forbidden
// edge. Edges with an end outside every layer are not evaluated.
func (p Policy) forbids(fromDir, toDir string) bool {
	from, fromOK := p.layerOf(fromDir)
	to, toOK := p.layerOf(toDir)
	return fromOK && toOK && from != to && p.forbidden[[2]string{from, to}]
}
