package configauthoring

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

func TestValidateLayerPrefixCandidate_AcceptsASiblingThatSharesAStemWithAnExistingPrefix(t *testing.T) {
	existing := []projectconfig.Layer{{Name: "core", Prefixes: []string{"core"}}}

	if err := validateLayerPrefixCandidate("utils", []string{"core-utils"}, existing); err != nil {
		t.Fatalf("core-utils is a sibling of core, not a descendant, so it must be accepted; got %v", err)
	}
}

func TestValidateLayerPrefixCandidate_RejectsADescendantWhenAStemSiblingIsAlsoDeclared(t *testing.T) {
	existing := []projectconfig.Layer{
		{Name: "core", Prefixes: []string{"core"}},
		{Name: "utils", Prefixes: []string{"core-utils"}},
	}

	err := validateLayerPrefixCandidate("src", []string{"core/src"}, existing)

	if err == nil || !strings.Contains(err.Error(), "non-overlapping") {
		t.Fatalf("core/src sits under core, so it must be rejected as overlapping while core-utils is also declared; got %v", err)
	}
}
