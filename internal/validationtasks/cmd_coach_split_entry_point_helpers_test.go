package validationtasks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var suiteEntryPoint = regexp.MustCompile(`(?s)func (Test\w+)\(t \*testing\.T\) \{[^}]*?\bRunSpecs\(`)

// runSpecsEntryPoints names the Test function that holds each RunSpecs call of the Go test files in dir.
func runSpecsEntryPoints(dir string) []string {
	GinkgoHelper()
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	Expect(err).NotTo(HaveOccurred())
	var names []string
	for _, file := range files {
		names = append(names, entryPointsOf(file)...)
	}
	return names
}

func entryPointsOf(file string) []string {
	GinkgoHelper()
	source := codeWithoutLineComments(file)
	var names []string
	matches := suiteEntryPoint.FindAllStringSubmatch(source, -1)
	for _, match := range matches {
		names = append(names, match[1])
	}
	if calls := strings.Count(source, "RunSpecs("); calls != len(matches) {
		names = append(names, filepath.Base(file)+" calls RunSpecs outside a Test function")
	}
	return names
}

func codeWithoutLineComments(file string) string {
	GinkgoHelper()
	raw, err := os.ReadFile(file)
	Expect(err).NotTo(HaveOccurred())
	var code []string
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			code = append(code, line)
		}
	}
	return strings.Join(code, "\n")
}
