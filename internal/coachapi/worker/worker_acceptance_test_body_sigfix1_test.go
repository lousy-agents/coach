package worker_test

import (
	"go/ast"
	"os"
	"strings"

	. "github.com/onsi/gomega"
)

type sigbodyworkerAcceptanceTesthasNoDirectRedisOrSQSClientImport struct {
	banned []string
	e      os.
		DirEntry
	f *ast.
		File
}

func (sigRecv *sigbodyworkerAcceptanceTesthasNoDirectRedisOrSQSClientImport) call() {

	for _, imp := range sigRecv.f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		for _, b := range sigRecv.banned {
			Expect(path).NotTo(HavePrefix(b), "%s must not import %s (use queue.TaskQueue only)", sigRecv.e.Name(), b)
		}

		Expect(path).NotTo(ContainSubstring("/queue/redisstream"), sigRecv.e.Name())
		Expect(path).NotTo(ContainSubstring("/queue/sqs"), sigRecv.e.Name())
	}
}
