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

func tsconfigJSON(v any) *fstest.MapFile {
	data, err := json.Marshal(v)
	Expect(err).NotTo(HaveOccurred())
	return &fstest.MapFile{Data: data}
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

// readJSSemanticsTypescriptDevDependency reads js/semantics/package.json's
// devDependencies.typescript version string directly from disk so the
// real-sidecar coverage-identity spec cannot drift from the actual pinned
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

// copyDirRecursive copies every file and directory under src into dst,
// used by the coverage-identity spec to mutate a private copy of the
// vendored sidecar tree without touching the real build output.
func copyDirRecursive(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, p)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, content, 0o644)
	})
}
