package projectmodel_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/domain"
)

// diagnosticCodes lists every string constant named Diag* that the non-test sources of the given package
// directories declare with a literal value: the codes a project coverage can carry.
func declaredDiagnosticCodes(directories ...string) map[string]string {
	GinkgoHelper()
	codes := map[string]string{}
	for _, directory := range directories {
		packages, err := parser.ParseDir(token.NewFileSet(), directory, func(info fs.FileInfo) bool { return !strings.HasSuffix(info.Name(), "_test.go") }, 0)
		Expect(err).NotTo(HaveOccurred())
		for _, pkg := range packages {
			for _, file := range pkg.Files {
				collectDiagnosticCodes(file, codes)
			}
		}
	}
	return codes
}

func collectDiagnosticCodes(file *ast.File, codes map[string]string) {
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}
		for _, spec := range general.Specs {
			value := spec.(*ast.ValueSpec)
			for i, name := range value.Names {
				literal, isLiteral := value.Values[min(i, len(value.Values)-1)].(*ast.BasicLit)
				if strings.HasPrefix(name.Name, "Diag") && isLiteral && literal.Kind == token.STRING {
					code, _ := strconv.Unquote(literal.Value)
					codes[name.Name] = code
				}
			}
		}
	}
}

var _ = Describe("every project diagnostic code carries a message (#894)", func() {
	It("finds the codes the two packages declare", func() {
		codes := declaredDiagnosticCodes(".", "../domain")

		Expect(codes).To(HaveKeyWithValue("DiagLayerBypassAmbiguousLayer", "project_layer_bypass_ambiguous_layer"))
		Expect(codes).To(HaveKeyWithValue("DiagLayerBypassBudgetExceeded", "project_layer_bypass_budget_exceeded"))
		Expect(len(codes)).To(BeNumerically(">=", 25))
	})

	It("gives every declared code a non-empty message when a coverage is canonicalised without one", func() {
		for name, code := range declaredDiagnosticCodes(".", "../domain") {
			coverage := domain.CanonicalCoverage(domain.Coverage{Diagnostics: []domain.Diagnostic{{Code: code}}})

			Expect(coverage.Diagnostics).To(HaveLen(1))
			Expect(coverage.Diagnostics[0].Message).NotTo(BeEmpty(), "%s (%s) has no message", name, code)
		}
	})

	It("keeps a message a diagnostic already carries", func() {
		coverage := domain.CanonicalCoverage(domain.Coverage{Diagnostics: []domain.Diagnostic{{Code: domain.DiagLayerBypassAmbiguousLayer, Message: "its own words"}}})

		Expect(coverage.Diagnostics[0].Message).To(Equal("its own words"))
	})
})
