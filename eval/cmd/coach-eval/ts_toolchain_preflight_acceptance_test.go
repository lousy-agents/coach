package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// checkoutWithCompiler is a checkout skeleton whose js/semantics/node_modules/.bin/tsc is a script that prints the
// given text, or has no compiler at all when script is empty.
func checkoutWithCompiler(script string) string {
	root := GinkgoT().TempDir()
	bin := filepath.Join(root, "js", "semantics", "node_modules", ".bin")
	Expect(os.MkdirAll(bin, 0o755)).To(Succeed())
	if script != "" {
		Expect(os.WriteFile(filepath.Join(bin, "tsc"), []byte("#!/bin/sh\n"+script+"\n"), 0o755)).To(Succeed())
	}
	return root
}

var _ = Describe("the TypeScript specs' toolchain preflight (#852)", func() {
	It("refuses a checkout whose compiler is missing with one message that names js-install and the cause", func() {
		_, err := checkTypeScriptToolchain(checkoutWithCompiler(""))

		Expect(err).To(MatchError(And(
			ContainSubstring("js/semantics/node_modules/.bin/tsc"),
			ContainSubstring("mise run js-install"),
			ContainSubstring("while the specs run"),
		)))
	})

	It("refuses a compiler of another version, naming both versions", func() {
		_, err := checkTypeScriptToolchain(checkoutWithCompiler("echo 'Version 5.9.2'"))

		Expect(err).To(MatchError(And(ContainSubstring("5.9.2"), ContainSubstring(pinnedTypeScript))))
	})

	It("refuses a compiler that prints no version", func() {
		_, err := checkTypeScriptToolchain(checkoutWithCompiler("exit 0"))

		Expect(err).To(MatchError(ContainSubstring(pinnedTypeScript)))
	})

	It("accepts the installed pinned compiler and returns its path", func() {
		tsc, err := checkTypeScriptToolchain(repositoryRoot())

		Expect(err).NotTo(HaveOccurred())
		Expect(tsc).To(Equal(filepath.Join(repositoryRoot(), "js", "semantics", "node_modules", ".bin", "tsc")))
	})

	It("snapshots the compiler into its own directory, so that no later install of the checkout can swap it", func() {
		original, err := checkTypeScriptToolchain(repositoryRoot())
		Expect(err).NotTo(HaveOccurred())
		into := GinkgoT().TempDir()

		snapshot, err := snapshotCompiler(original, into)

		Expect(err).NotTo(HaveOccurred())
		resolved, resolveErr := filepath.EvalSymlinks(snapshot)
		Expect(resolveErr).NotTo(HaveOccurred())
		intoResolved, _ := filepath.EvalSymlinks(into)
		Expect(resolved).To(HavePrefix(intoResolved), "the snapshot is a copy, not a link into the checkout")
		output, runErr := exec.Command(snapshot, "--version").CombinedOutput()
		Expect(runErr).NotTo(HaveOccurred(), string(output))
		Expect(strings.TrimSpace(string(output))).To(ContainSubstring(pinnedTypeScript))
	})
})
