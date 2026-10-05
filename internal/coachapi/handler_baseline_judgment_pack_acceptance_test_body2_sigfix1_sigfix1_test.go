package coachapi_test

import . "github.com/onsi/gomega"

type sigcallS6 struct {
	contents  []string
	v61725567 *sigbodyhandlerBaselineJudgmentPackAcceptanceTestembedsSpanWi
}

func (sigRecv *sigcallS6) call() {

	for _, content := range sigRecv.contents {
		if content == "" {
			continue
		}
		*sigRecv.v61725567.checked++

		Expect(content).To(MatchRegexp(`(?m)^[> ]\s*\d+\|`),
			"evidence should be FormatSpanWindow-numbered, not raw full file")
		Expect(content).NotTo(ContainSubstring("pad-line-79-unique-marker-FULLFILE"),
			"default evidence must not embed the entire padded file")
	}
}
