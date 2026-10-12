// Package tstestutil holds helpers shared by Ginkgo acceptance tests that
// copy this repository's installed TypeScript compiler into a fixture
// worktree. Production code must not import it.
package tstestutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/gomega"
)

// TypeScriptVersion reads js/semantics' own installed typescript
// devDependency version directly from disk, so fixtures declaring an exact
// matching version in package.json cannot drift from the actual installed
// copy the suite copies.
func TypeScriptVersion() string {
	data, err := os.ReadFile(filepath.Join(typescriptDir(), "package.json"))
	Expect(err).NotTo(HaveOccurred())
	var manifest struct {
		Version string `json:"version"`
	}
	Expect(json.Unmarshal(data, &manifest)).To(Succeed())
	Expect(manifest.Version).NotTo(BeEmpty())
	return manifest.Version
}

// NPMArch maps Go's runtime.GOARCH to the npm/Node process.arch naming
// convention typescript's native platform package directories use (e.g.
// "amd64" -> "x64"), which differs from Go's own arch names.
func NPMArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "386":
		return "ia32"
	default:
		return runtime.GOARCH
	}
}

func repoRoot() string {
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue(), "runtime.Caller(0) failed")
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

func typescriptDir() string {
	return filepath.Join(repoRoot(), "js", "semantics", "node_modules", "typescript")
}

func nativeTypescriptDir() string {
	name := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, NPMArch())
	return filepath.Join(repoRoot(), "js", "semantics", "node_modules", "@typescript", name)
}
