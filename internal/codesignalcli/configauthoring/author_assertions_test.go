package configauthoring

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalLayers(a, b []projectconfig.Layer) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || !equalStringSlices(a[i].Prefixes, b[i].Prefixes) {
			return false
		}
	}
	return true
}

func equalForbiddenImports(a, b []projectconfig.ForbiddenImport) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
