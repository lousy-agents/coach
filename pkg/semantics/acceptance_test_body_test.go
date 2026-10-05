package semantics_test

import (
	"context"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_acceptanceTest_ordersImportsAndFindingsByDocumentPositionAC110_68(analyzer *semantics.Analyzer) {
	source := []byte(`package main

import (
	"os"
	"fmt"
)

func NewZeta() *int {
	fmt.Println(os.Args)
	return nil
}

func NewAlpha() *int {
	return nil
}
`)

	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "main.go",
		Language: semantics.LanguageGo,
		Content:  source,
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(result.Imports).To(HaveLen(2))
	Expect(result.Imports[0].Location.StartByte).To(BeNumerically("<=", result.Imports[1].Location.StartByte))

	Expect(result.Findings).NotTo(BeEmpty())
	for i := 1; i < len(result.Findings); i++ {
		Expect(result.Findings[i].Location.StartByte).To(BeNumerically(">=", result.Findings[i-1].Location.StartByte))
	}
	names := map[string]bool{}
	for _, f := range result.Findings {
		names[f.Name] = true
	}
	Expect(names).To(HaveKey("NewZeta"))
	Expect(names).To(HaveKey("NewAlpha"))

	Expect(result.Findings[0].Name).To(Equal("NewZeta"))
}
