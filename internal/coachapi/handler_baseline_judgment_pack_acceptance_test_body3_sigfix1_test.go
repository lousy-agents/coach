package coachapi_test

import (
	"strings"

	"github.com/lousy-agents/coach/internal/coachapi"
	. "github.com/onsi/gomega"
)

type sigbodyhandlerBaselineJudgmentPackAcceptanceTestpersistsAgen struct {
	d coachapi.
		JobDiagnostic
	msg           string
	sawBudgetDiag *bool
	scope         string
}

func (sigRecv *sigbodyhandlerBaselineJudgmentPackAcceptanceTestpersistsAgen) call() {

	if strings.Contains(sigRecv.msg, "judgment_budget_exceeded") ||
		(strings.Contains(sigRecv.msg, "judged=") && strings.Contains(sigRecv.msg, "remaining=")) ||
		(strings.Contains(sigRecv.scope, "judgment") && strings.Contains(sigRecv.scope, "budget")) {
		*sigRecv.sawBudgetDiag = true

		if strings.Contains(sigRecv.d.Message, "judged=") {
			Expect(sigRecv.d.Message).To(ContainSubstring("remaining="))
		}
	}
}
