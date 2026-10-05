package modelgateway_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

type sigbodygatewayAcceptanceTestisTheOnlyNonTestPackageThatOwnsT struct {
	offenders *[]string
	root      string
}

func (sigRecv *sigbodygatewayAcceptanceTestisTheOnlyNonTestPackageThatOwnsT) call(path string, d os.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if d.IsDir() {
		base := d.Name()
		if base == ".git" || base == "node_modules" || base == "vendor" || base == "dist" || base == "dist-test" {
			return filepath.SkipDir
		}
		return nil
	}
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return nil
	}
	rel, relErr := filepath.Rel(sigRecv.root, path)
	Expect(relErr).NotTo(HaveOccurred())
	if strings.HasPrefix(rel, "internal"+string(filepath.Separator)+"modelgateway"+string(filepath.Separator)) {
		return nil
	}
	raw, readErr := os.ReadFile(path)
	if readErr != nil {
		return readErr
	}
	if strings.Contains(string(raw), "/v1/chat/completions") {
		*sigRecv.offenders = append(*sigRecv.offenders, rel)
	}
	return nil
}
