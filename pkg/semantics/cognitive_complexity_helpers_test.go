package semantics_test

import (
	"context"

	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func ccByName(records []semantics.FunctionCognitiveComplexity, name string) (semantics.FunctionCognitiveComplexity, bool) {
	for _, r := range records {
		if r.Name == name {
			return r, true
		}
	}
	return semantics.FunctionCognitiveComplexity{}, false
}

// goPackagePrefix wraps a Go function body fixture so AnalyzeBytes sees a
// valid compilation unit (package + imports used by the worked examples).
func goPackagePrefix(body string) []byte {
	return []byte("package main\n\nimport \"fmt\"\n\n" + body)
}

func analyzeGoCC(analyzer *semantics.Analyzer, body string) semantics.Result {
	GinkgoHelper()
	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "example.go",
		Language: semantics.LanguageGo,
		Content:  goPackagePrefix(body),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result).NotTo(BeNil())
	Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
	return *result
}

func analyzeTSCC(analyzer *semantics.Analyzer, source string) semantics.Result {
	GinkgoHelper()
	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "example.ts",
		Language: semantics.LanguageTypeScript,
		Content:  []byte(source),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result).NotTo(BeNil())
	Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
	return *result
}
