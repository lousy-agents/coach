package semantics_test

import (
	"github.com/lousy-agents/coach/pkg/semantics"

	. "github.com/onsi/gomega"
)

type sigbodyacceptanceTestwhenSourceContainsEveryGoImportFormAC31 struct {
	result *semantics.
		Result
}

func (sigRecv *sigbodyacceptanceTestwhenSourceContainsEveryGoImportFormAC31) call(wantPath, wantAlias string) {
	var found *semantics.ImportFeature
	for i := range sigRecv.result.Imports {
		if sigRecv.result.Imports[i].Path == wantPath {
			found = &sigRecv.result.Imports[i]
			break
		}
	}
	Expect(found).NotTo(BeNil(), "expected an import with path %q, got %+v", wantPath, sigRecv.result.Imports)
	Expect(found.Alias).To(Equal(wantAlias))
}
