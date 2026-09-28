package projectmodel_test

import (
	"encoding/json"

	. "github.com/onsi/gomega"
)

func body_acceptanceTest_omitsEveryOmitemptyKeyFromTheMarshaledOutput_154() {
	model := baseModel()

	out, err := json.Marshal(model)
	Expect(err).NotTo(HaveOccurred())
	text := string(out)

	for _, key := range []string{
		`"repository"`, `"workspaces"`, `"modules"`, `"packages"`,
		`"files"`, `"import_edges"`, `"call_facts"`, `"reachability_facts"`,
	} {
		Expect(text).NotTo(ContainSubstring(key), "expected %s to be omitted from %s", key, text)
	}

	var top map[string]json.RawMessage
	Expect(json.Unmarshal(out, &top)).To(Succeed())
	var coverage map[string]json.RawMessage
	Expect(json.Unmarshal(top["coverage"], &coverage)).To(Succeed())
	Expect(coverage).NotTo(HaveKey("diagnostics"), "expected Coverage.Diagnostics to be omitted from %s", coverage)
}
