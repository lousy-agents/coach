package validationtasks

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const rootSkipCovers = "mise run acceptance-ci-env"

// rootSkipReasons returns, for every test that skips when it runs as root, the
// file, line and text of the skip. A root skip is the first Skip call within a
// few lines after an `os.Geteuid() == 0` check.
func rootSkipReasons(root string) []string {
	var found []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return skipVendoredDirectory(entry.Name())
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, skip := range rootSkipsIn(string(data)) {
			found = append(found, path+":"+skip)
		}
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
	return found
}

func skipVendoredDirectory(name string) error {
	if name == "node_modules" || name == ".git" {
		return filepath.SkipDir
	}
	return nil
}

// rootSkipsIn returns the Skip call within three lines after each root check in source.
func rootSkipsIn(source string) []string {
	var skips []string
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		if strings.Contains(line, "os.Geteuid() == 0") {
			if skip, ok := skipAfter(lines[i+1:]); ok {
				skips = append(skips, skip)
			}
		}
	}
	return skips
}

func skipAfter(lines []string) (string, bool) {
	for _, line := range lines[:min(3, len(lines))] {
		if strings.Contains(line, "Skip(") {
			return strings.TrimSpace(line), true
		}
	}
	return "", false
}

var _ = Describe("a test that cannot build its state as root (#733 AC6)", func() {
	It("#733 AC6: names `mise run acceptance-ci-env` in its root skip reason, because that task covers the non-root case", func() {
		skips := rootSkipReasons(filepath.Join("..", ".."))
		Expect(skips).NotTo(BeEmpty(), "sanity: the repository has root skips")

		var missing []string
		for _, skip := range skips {
			if !strings.Contains(skip, rootSkipCovers) {
				missing = append(missing, skip)
			}
		}
		Expect(missing).To(BeEmpty(), "each root skip must say that %q runs it as non-root", rootSkipCovers)
	})
})
