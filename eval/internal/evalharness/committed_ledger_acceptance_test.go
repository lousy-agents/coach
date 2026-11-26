package evalharness_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalharness"
)

var _ = Describe("the run ledger committed to the repository", func() {
	It("loads line by line under the current schemas, so every tool that reads it can read it", func() {
		lines, err := evalharness.LoadLedger("../../ledger/runs.jsonl")

		Expect(err).NotTo(HaveOccurred())
		Expect(lines).NotTo(BeEmpty())
	})
})

var _ = Describe("the unplaced reasons a history rule record carries, as the ledger loader validates them", func() {
	// loadWithFirstRuleField loads the committed history line with its first rule's unplaced_reasons set to value,
	// or removed when set is false, and returns the loader's error.
	loadWithFirstRuleField := func(value any, set bool) error {
		raw, err := os.ReadFile("../../ledger/runs.jsonl")
		Expect(err).NotTo(HaveOccurred())
		var history map[string]any
		for _, text := range strings.Split(string(raw), "\n") {
			if strings.TrimSpace(text) == "" {
				continue
			}
			var line map[string]any
			Expect(json.Unmarshal([]byte(text), &line)).To(Succeed())
			if line["kind"] == "history" {
				history = line
				break
			}
		}
		Expect(history).NotTo(BeNil())
		run := history["run"].(map[string]any)
		repo := run["repositories"].([]any)[0].(map[string]any)
		rule := repo["rules"].([]any)[0].(map[string]any)
		if set {
			rule["unplaced_reasons"] = value
		} else {
			delete(rule, "unplaced_reasons")
		}
		encoded, err := json.Marshal(history)
		Expect(err).NotTo(HaveOccurred())
		path := filepath.Join(GinkgoT().TempDir(), "runs.jsonl")
		Expect(os.WriteFile(path, append(encoded, '\n'), 0o644)).To(Succeed())
		_, err = evalharness.LoadLedger(path)
		return err
	}

	It("accepts unplaced reasons that are non-empty strings", func() {
		Expect(loadWithFirstRuleField([]string{"language not in the Go history inventory"}, true)).To(Succeed())
	})

	It("accepts a rule record with no unplaced reasons, as the lines written before the field are", func() {
		Expect(loadWithFirstRuleField(nil, false)).To(Succeed())
	})

	It("refuses unplaced reasons that are not an array", func() {
		Expect(loadWithFirstRuleField("language not in the Go history inventory", true)).NotTo(Succeed())
	})

	It("refuses an empty reason", func() {
		Expect(loadWithFirstRuleField([]string{""}, true)).NotTo(Succeed())
	})
})
