package semantics_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func expectImportWithAlias(result *semantics.Result, wantPath, wantAlias string) {
	var found *semantics.ImportFeature
	for i := range result.Imports {
		if result.Imports[i].Path == wantPath {
			found = &result.Imports[i]
			break
		}
	}
	Expect(found).NotTo(BeNil(), "expected an import with path %q, got %+v", wantPath, result.Imports)
	Expect(found.Alias).To(Equal(wantAlias))
}
