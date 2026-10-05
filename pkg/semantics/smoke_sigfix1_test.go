package semantics

import "github.com/lousy-agents/coach/pkg/semantics/internal/engine"

type sigTestSmokeTSGrammarExposesExpectedTightCouplingNodeKinds21 struct {
	hasConstructorField *bool
}

func (sigRecv *sigTestSmokeTSGrammarExposesExpectedTightCouplingNodeKinds21) call(n engine.Node) {
	if n == nil {
		return
	}
	if n.Kind() == "new_expression" && n.ChildByFieldName("constructor") != nil {
		*sigRecv.hasConstructorField = true
	}
	for i := 0; i < n.ChildCount(); i++ {
		sigRecv.call(n.Child(i))
	}
}
