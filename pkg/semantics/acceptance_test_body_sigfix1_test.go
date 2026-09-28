package semantics_test

import (
	"github.com/lousy-agents/coach/pkg/semantics"

	. "github.com/onsi/gomega"
)

type sigbodyacceptanceTestwhenSourceContainsEveryGoImportFormAC31 struct {
	result **semantics.Result
}

func (sigRecv *sigbodyacceptanceTestwhenSourceContainsEveryGoImportFormAC31) call(wantPath, wantAlias string) {
	res := *sigRecv.result
	var found *semantics.ImportFeature
	for i := range res.Imports {
		if res.Imports[i].Path == wantPath {
			found = &res.Imports[i]
			break
		}
	}
	Expect(found).NotTo(BeNil(), "expected an import with path %q, got %+v", wantPath, res.Imports)
	Expect(found.Alias).To(Equal(wantAlias))
}
