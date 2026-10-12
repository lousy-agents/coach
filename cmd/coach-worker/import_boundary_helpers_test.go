package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/gomega"
)

// expectNoDirectBrokerClientImports parses every production file in this
// package and rejects direct go-redis / AWS SDK imports.
func expectNoDirectBrokerClientImports() {
	// Composition root may import redisstream (adapter); it must not
	// reach for go-redis / aws-sdk directly.
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())
	dir := filepath.Dir(thisFile)

	bannedDirect := []string{
		"github.com/redis/go-redis",
		"github.com/aws/aws-sdk-go",
		"github.com/aws/aws-sdk-go-v2",
	}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	Expect(err).NotTo(HaveOccurred())
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		Expect(err).NotTo(HaveOccurred(), path)
		expectFileImportsNoDirect(e.Name(), f, bannedDirect)
	}
}

// expectFileImportsNoDirect requires that file name imports no package under
// any of the banned prefixes.
func expectFileImportsNoDirect(name string, f *ast.File, banned []string) {
	for _, imp := range f.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		for _, b := range banned {
			Expect(p).NotTo(HavePrefix(b), "%s must not import %s directly", name, b)
		}
	}
}
