package worker_test

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

func expectNoDirectBrokerClientImports() {
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())
	dir := filepath.Dir(thisFile)

	entries, err := os.ReadDir(dir)
	Expect(err).NotTo(HaveOccurred())

	banned := []string{
		"github.com/redis/go-redis",
		"github.com/ThreeDotsLabs/watermill-redisstream",
		"github.com/ThreeDotsLabs/watermill-aws",
		"github.com/aws/aws-sdk-go",
		"github.com/aws/aws-sdk-go-v2",
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		Expect(err).NotTo(HaveOccurred(), path)
		expectFileImportsNoBrokerClient(e.Name(), f, banned)
	}
}

// expectFileImportsNoBrokerClient requires that file name imports neither a
// banned broker client nor a concrete queue adapter package.
func expectFileImportsNoBrokerClient(name string, f *ast.File, banned []string) {
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		for _, b := range banned {
			Expect(path).NotTo(HavePrefix(b), "%s must not import %s (use queue.TaskQueue only)", name, b)
		}
		// Adapter packages are also banned inside worker proper.
		Expect(path).NotTo(ContainSubstring("/queue/redisstream"), name)
		Expect(path).NotTo(ContainSubstring("/queue/sqs"), name)
	}
}
