package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/gomega"
)

func body_mainAcceptanceTest_doesNotImportRedisSQSClientsOutsideTheQueueAdapt_99() {
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
		(&sigbodymainAcceptanceTestdoesNotImportRedisSQSClientsOutside{bannedDirect: bannedDirect, e: e, f: f}).call()

	}
}
