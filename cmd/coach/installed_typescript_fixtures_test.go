package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/gomega"
)

func writeInstalledTypescriptCompilerUnder(repo, relDir, version string) {
	if _, err := os.Stat(filepath.Join(repo, ".gitignore")); err != nil {
		commitFile(repo, ".gitignore", "node_modules\n")
	}
	manifest := "node_modules/typescript/package.json"
	if relDir != "." && relDir != "" {
		manifest = relDir + "/" + manifest
	}
	writeWorktreeFile(repo, manifest, fmt.Sprintf(`{"name":"typescript","version":%q}`+"\n", version))
}

func writeInstalledNativeTypescriptUnder(repo, relDir, version string) {
	unscoped := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
	manifest := filepath.Join("node_modules", "@typescript", unscoped, "package.json")
	if relDir != "." && relDir != "" {
		manifest = relDir + "/" + manifest
	}
	writeWorktreeFile(repo, manifest, fmt.Sprintf(`{"name":%q,"version":%q}`+"\n", "@typescript/"+unscoped, version))
}

// writeWorktreeFile writes name with contents directly into the worktree at
// repo without committing or `git add`ing it. The compiler check reads
// package.json/mise.toml/node_modules as host-readiness state of the
// worktree, never the Git snapshot, so these fixtures deliberately stay
// uncommitted -- proving the resolver reads the worktree directly rather
// than depending on anything reaching HEAD.
func writeWorktreeFile(repo, name, contents string) {
	full := filepath.Join(repo, name)
	Expect(os.MkdirAll(filepath.Dir(full), 0o755)).To(Succeed())
	Expect(os.WriteFile(full, []byte(contents), 0o644)).To(Succeed())
}

func writeInstalledTypescript(repo, version string) {
	writeInstalledTypescriptUnder(repo, ".", version)
}

func writeInstalledTypescriptCompilerOnly(repo, version string) {
	writeInstalledTypescriptCompilerUnder(repo, ".", version)
}

func writeInstalledTypescriptUnder(repo, relDir, version string) {
	writeInstalledTypescriptCompilerUnder(repo, relDir, version)
	writeInstalledNativeTypescriptUnder(repo, relDir, version)
}
