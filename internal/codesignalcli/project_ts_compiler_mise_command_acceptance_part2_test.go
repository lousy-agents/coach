package codesignalcli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

// anyFileNamed: a missing root is not an error -- it simply contains
// nothing.
func anyFileNamed(root, name string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && d.Name() == name {
			found = true
		}
		return nil
	})
	return found
}

// npmGlobalStyleInstallPresent reports whether root contains an
// npm-global-style "lib/node_modules/<name>" layout anywhere under it -- the
// shape only mise's npm.shell_out=true backend's real `npm install -g`
// produces, never the default aube backend's own installer. It distinguishes
// a genuine invocation of the shell_out=true branch from a config.toml typo
// that silently fell back to the default backend, which would still pass a
// lifecycle-script-suppression assertion trivially (it never runs lifecycle
// scripts at all, regardless of this package's own suppression env var).
func npmGlobalStyleInstallPresent(root, name string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || d.Name() != "lib" {
			return nil
		}
		if _, statErr := os.Stat(filepath.Join(path, "node_modules", name)); statErr == nil {
			found = true
		}
		return nil
	})
	return found
}

// filteredEnviron returns os.Environ() with every entry whose key equals key
// removed, so a positive control that must run genuinely unsuppressed cannot
// accidentally inherit that suppression from this process's own ambient
// environment.
func filteredEnviron(key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func writeHomeNpmrcRegistry(registryURL string) {
	home := os.Getenv("HOME")
	Expect(home).NotTo(BeEmpty(), "freshMiseInstallEnv must run before writing HOME/.npmrc")
	if !strings.HasSuffix(registryURL, "/") {
		registryURL += "/"
	}
	Expect(os.WriteFile(filepath.Join(home, ".npmrc"), []byte("registry="+registryURL+"\n"), 0o644)).To(Succeed())
}
