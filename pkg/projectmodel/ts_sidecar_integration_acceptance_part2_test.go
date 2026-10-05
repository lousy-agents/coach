package projectmodel_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"testing/fstest"

	. "github.com/onsi/gomega"
)

// pathExcludingExecutables returns the current process's PATH with any
// directory containing one of names removed, used by the "missing runtime"
// spec below to construct a child environment where the sidecar's
// `#!/usr/bin/env node` shebang cannot locate node.
func pathExcludingExecutables(names ...string) string {
	var kept []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		excluded := false
		(&sigpathExcludingExecutablesS2{dir: dir, excluded: &excluded, names: names}).call()

		if !excluded {
			kept = append(kept, dir)
		}
	}
	return strings.Join(kept, string(os.PathListSeparator))
}

// ensureRealTSSidecarBinary builds the real compiled sidecar
// (`npm run build:project-sidecar`, i.e. `mise run project-sidecar-build`)
// at most once per test binary run and memoizes the outcome. It cannot be a
// second Ginkgo BeforeSuite -- Ginkgo permits exactly one per suite, and
// ts_sidecar_acceptance_test.go (frozen, Task 1) already declares the
// suite's only one, for the fake sidecar -- so every spec below calls this
// from its own BeforeEach and Skips with skipReason when Node/npm are
// unavailable or the build fails, degrading this suite gracefully instead
// of failing Go-only environments (issue #214's explicit requirement).
func ensureRealTSSidecarBinary() (path string, skipReason string) {
	realTSSidecarOnce.Do(func() {
		body_tsSidecarIntegrationAcceptancePart2Test_55()
	})
	return realTSSidecarPath, realTSSidecarSkip
}

// sidecarSourceDigest hashes the sorted relative path and content of every
// .js file under vendoredDir (the real sidecar implementation copied to
// bin/project-sidecar/ by scripts/build-project-sidecar.mjs). This is
// deliberately not a hash of bin/coach-ts-project-sidecar itself: that
// file is a fixed-size, hand-written ESM shim whose bytes never change
// across sidecar versions (see the Implementer Report), so hashing it
// would produce a "version identity" that never actually varies with the
// sidecar's version.
func sidecarSourceDigest(vendoredDir string) string {
	var relPaths []string
	Expect(filepath.WalkDir(vendoredDir, func(p string, d fs.DirEntry, err error) error {
		Expect(err).NotTo(HaveOccurred())
		if d.IsDir() || !strings.HasSuffix(p, ".js") {
			return nil
		}
		rel, relErr := filepath.Rel(vendoredDir, p)
		Expect(relErr).NotTo(HaveOccurred())
		relPaths = append(relPaths, filepath.ToSlash(rel))
		return nil
	})).To(Succeed())
	sort.Strings(relPaths)

	h := sha256.New()
	for _, rel := range relPaths {
		content, readErr := os.ReadFile(filepath.Join(vendoredDir, filepath.FromSlash(rel)))
		Expect(readErr).NotTo(HaveOccurred())
		fmt.Fprintf(h, "%s\x00", rel)
		h.Write(content)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// repoRootFromThisFile locates the repository root relative to this test
// file's own path, mirroring pgMigrationFiles' runtime.Caller(0) convention
// in internal/coachapi/store_postgres_acceptance_test.go.
func repoRootFromThisFile() string {
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue(), "runtime.Caller(0) failed")
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

func jsSemanticsRoot() string {
	return filepath.Join(repoRootFromThisFile(), "js", "semantics")
}

func file(content string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(content)}
}

// readJSSemanticsTypescriptDevDependency reads js/semantics/package.json's
// devDependencies.typescript version string directly from disk so the
// coverage-identity spec above cannot drift from the actual pinned
// toolchain version.
func readJSSemanticsTypescriptDevDependency() string {
	data, err := os.ReadFile(filepath.Join(jsSemanticsRoot(), "package.json"))
	Expect(err).NotTo(HaveOccurred())

	var pkg struct {
		DevDependencies struct {
			TypeScript string `json:"typescript"`
		} `json:"devDependencies"`
	}
	Expect(json.Unmarshal(data, &pkg)).To(Succeed())
	Expect(pkg.DevDependencies.TypeScript).NotTo(BeEmpty())
	return pkg.DevDependencies.TypeScript
}
