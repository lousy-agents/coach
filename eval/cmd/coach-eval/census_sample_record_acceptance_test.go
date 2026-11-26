package main_test

import (
	"encoding/json"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// setSample rewrites the first repository's sample in the corpus manifest at path.
func setSample(path string, seed int64) {
	raw, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	var manifest map[string]any
	Expect(json.Unmarshal(raw, &manifest)).To(Succeed())
	manifest["repositories"].([]any)[0].(map[string]any)["sample"] = map[string]any{"seed": seed}
	out, err := json.Marshal(manifest)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(path, out, 0o644)).To(Succeed())
}

var _ = Describe("the sample a census records", func() {
	It("records the manifest's seed in the repository of its ledger line", func() {
		busyPath, busySHA := censusRepo(goRepoFiles(complexFunctions(2)))
		corpus := writeCorpus(corpusRepo{ID: "busy", Path: busyPath, SHA: busySHA, Language: "go"})
		setSample(corpus, 9)

		outcome, _, outputs := runCensusInto(GinkgoT().TempDir(), corpus, coachBinary())
		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)

		ledger := jsonLines(outputs.ledger)
		repository := ledger[0]["run"].(map[string]any)["repositories"].([]any)[0].(map[string]any)
		Expect(repository["sample"]).To(Equal(map[string]any{"seed": float64(9)}))
	})
})
