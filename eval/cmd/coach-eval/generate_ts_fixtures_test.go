package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const pinnedTypeScript = "7.0.2"

func typeScriptCellFlags(cell programCell) []string {
	return append([]string{"--language", "typescript"}, cell.flags()...)
}

// repositoryRoot is the checkout that holds eval/ and js/semantics/, two levels above this package's directory.
func repositoryRoot() string {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	Expect(err).NotTo(HaveOccurred())
	return root
}

// installedTSC is the compiler the generator resolves by default; a spec that needs it fails, rather than skips, when it
// is missing, so that a run without the toolchain cannot look green.
func installedTSC() string {
	path := filepath.Join(repositoryRoot(), "js", "semantics", "node_modules", ".bin", "tsc")
	_, err := os.Stat(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "run `mise run js-install` first: the TypeScript specs use the pinned compiler")
	return path
}

func tscIn(dir string, args ...string) generateOutcome {
	cmd := exec.Command(installedTSC(), args...)
	cmd.Dir = dir
	return runGenerateCommand(cmd)
}

func nodeTest(dir string, files ...string) generateOutcome {
	cmd := exec.Command("node", append([]string{"--test", "--test-isolation=process"}, files...)...)
	cmd.Dir = dir
	return runGenerateCommand(cmd)
}

func filesMatching(tree map[string][]byte, pattern string) []string {
	matcher := regexp.MustCompile(pattern)
	files := []string{}
	for path := range tree {
		if matcher.MatchString(path) {
			files = append(files, path)
		}
	}
	return files
}

// stubTSC writes a compiler that only reports a version, to stand in for a wrong or missing pin.
func stubTSC(version string) string {
	dir := GinkgoT().TempDir()
	path := filepath.Join(dir, "tsc")
	script := fmt.Sprintf("#!/bin/sh\necho 'Version %s'\n", version)
	Expect(os.WriteFile(path, []byte(script), 0o755)).To(Succeed())
	return path
}

func runGenerateIn(dir, cacheHome string, extraEnv []string, args ...string) generateOutcome {
	cmd := exec.Command(coachEvalBinary, append([]string{"generate"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "XDG_CACHE_HOME="+cacheHome)
	cmd.Env = append(cmd.Env, extraEnv...)
	return runGenerateCommand(cmd)
}

var importSpecifier = regexp.MustCompile(`(?m)(?:\bfrom\s+|\bimport\s+)"([^"]+)"`)

func specifiersOf(source []byte) []string {
	specifiers := []string{}
	for _, match := range importSpecifier.FindAllSubmatch(source, -1) {
		specifiers = append(specifiers, string(match[1]))
	}
	return specifiers
}

func withoutTestOutput(outcome generateOutcome) string {
	return strings.TrimSpace(outcome.stdout + outcome.stderr)
}

func mustLookPath(name string) string {
	path, err := exec.LookPath(name)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return path
}

func pathFrom(dirs []string) string { return strings.Join(dirs, string(os.PathListSeparator)) }
