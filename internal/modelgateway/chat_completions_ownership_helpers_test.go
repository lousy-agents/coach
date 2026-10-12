package modelgateway_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

func expectOnlyModelgatewayOwnsChatCompletionsPath() {
	root := findModuleRoot()
	Expect(root).NotTo(BeEmpty())

	var offenders []string
	err := filepath.WalkDir(root, (&chatCompletionsPathScan{offenders: &offenders, root: root}).visit)
	Expect(err).NotTo(HaveOccurred())
	Expect(offenders).To(BeEmpty(), "chat-completions path must stay inside internal/modelgateway: %v", offenders)
}

// chatCompletionsPathScan records, through its visit WalkDirFunc, every
// production Go file outside internal/modelgateway that mentions the
// chat-completions path.
type chatCompletionsPathScan struct {
	offenders *[]string
	root      string
}

func (scan *chatCompletionsPathScan) visit(path string, d os.DirEntry, walkErr error) error {
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
	rel, relErr := filepath.Rel(scan.root, path)
	Expect(relErr).NotTo(HaveOccurred())
	if strings.HasPrefix(rel, "internal"+string(filepath.Separator)+"modelgateway"+string(filepath.Separator)) {
		return nil
	}
	raw, readErr := os.ReadFile(path)
	if readErr != nil {
		return readErr
	}
	if strings.Contains(string(raw), "/v1/chat/completions") {
		*scan.offenders = append(*scan.offenders, rel)
	}
	return nil
}

func findModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
