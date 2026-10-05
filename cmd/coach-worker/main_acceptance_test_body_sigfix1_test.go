package main

import (
	"go/ast"
	"os"
	"strings"

	. "github.com/onsi/gomega"
)

type sigbodymainAcceptanceTestdoesNotImportRedisSQSClientsOutside struct {
	bannedDirect []string
	e            os.
			DirEntry
	f *ast.
		File
}

func (sigRecv *sigbodymainAcceptanceTestdoesNotImportRedisSQSClientsOutside) call() {

	for _, imp := range sigRecv.f.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		for _, b := range sigRecv.bannedDirect {
			Expect(p).NotTo(HavePrefix(b), "%s must not import %s directly", sigRecv.e.Name(), b)
		}
	}
}
