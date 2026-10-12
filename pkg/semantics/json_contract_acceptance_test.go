package semantics_test

import (
	"context"
	"encoding/json"

	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the frozen JSON contract", func() {
	It("serializes a result with stable snake_case field names a consumer can persist and later read back (AC-4.1, AC-4.3)", func() {
		analyzer := mustAnalyzer()
		result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
			Path:     "main.go",
			Language: semantics.LanguageGo,
			Content:  []byte("package main\n\nfunc NewFoo() *int {\n\tif true {\n\t}\n\treturn nil\n}\n"),
		})
		Expect(err).NotTo(HaveOccurred())

		persisted, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())

		var asMap map[string]json.RawMessage
		Expect(json.Unmarshal(persisted, &asMap)).To(Succeed())
		Expect(asMap).To(HaveKey("path"))
		Expect(asMap).To(HaveKey("language"))
		Expect(asMap).To(HaveKey("parse_status"))
		Expect(asMap).To(HaveKey("metrics"))

		// A future consumer reading a persisted result back must recover the
		// same facts -- this is the promise the frozen JSON shape exists to
		// keep, not merely that marshaling succeeds.
		var roundTripped semantics.Result
		Expect(json.Unmarshal(persisted, &roundTripped)).To(Succeed())
		Expect(roundTripped).To(Equal(*result))
	})
})
