package codesignal_test

import (
	"encoding/json"

	. "github.com/onsi/gomega"
)

func body_reportShapeAcceptanceTest_marshalsEveryEntryWithTheFullAlwaysPresentKeySet_229(rawSignals []json.RawMessage) {
	for _, r := range rawSignals {
		var m map[string]json.RawMessage
		Expect(json.Unmarshal(r, &m)).To(Succeed())

		Expect(m).To(HaveKey("why_it_matters"))
		Expect(m).To(HaveKey("recommendation"))
		Expect(m).To(HaveKey("suggested_skill"))
		Expect(m).To(HaveKey("machine_evidence"))
		Expect(m).To(HaveKey("related_locations"))
		Expect(m).To(HaveKey("path_steps"))
		Expect(m).To(HaveKey("coverage_refs"))
	}
}
