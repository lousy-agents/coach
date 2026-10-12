package domain

import (
	"sort"
)

// ImportEdge is one resolved import from one package or file to another.
type ImportEdge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Site       string `json:"site,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

// CallFact is one statically resolved call edge.
type CallFact struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func canonicalImportEdges(in []ImportEdge) []ImportEdge {
	if len(in) == 0 {
		return in
	}
	out := append([]ImportEdge(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Site != b.Site {
			return a.Site < b.Site
		}
		return a.Resolution < b.Resolution
	})
	return out
}

func CanonicalCallFacts(in []CallFact) []CallFact {
	if len(in) == 0 {
		return in
	}
	out := append([]CallFact(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}
