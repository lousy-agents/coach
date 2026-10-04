package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

func body_projectTsSetupFailureAcceptanceTest_scopesTheResidueDisclosureToWorkingDirectoryNami_130() {
	repoRoot := newTempGitRepo()
	commitFile(repoRoot, ".gitignore", "node_modules/\n")
	commitFile(repoRoot, "packages/app/package.json", `{"name":"app","version":"1.0.0"}`+"\n")
	commitFile(repoRoot, "unrelated/tracked.txt", "original\n")

	// Dirt that has nothing to do with this setup run: a modified
	// tracked file and an untracked file, both outside
	// packages/app. If the residue read is not scoped to
	// WorkingDirectory, these leak into ChangedPaths.
	Expect(os.WriteFile(filepath.Join(repoRoot, "unrelated", "tracked.txt"), []byte("modified\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(repoRoot, "unrelated", "untracked.txt"), []byte("new\n"), 0o644)).To(Succeed())

	appDir := filepath.Join(repoRoot, "packages", "app")
	stubDir := writeFailingSetupExecutableWithResidue("npm")
	GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

	preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, projectPackageManager("npm"), appDir)
	Expect(err).NotTo(HaveOccurred())

	outcome, err := codesignalcli.RunConfirmedSetup(context.Background(), preview, true)
	Expect(err).NotTo(HaveOccurred())
	Expect(outcome.Kind).To(Equal(codesignalcli.SetupOutcomeFailed))
	Expect(outcome.ChangedPaths).NotTo(BeEmpty())

	joined := strings.Join(outcome.ChangedPaths, "\n")
	Expect(joined).To(ContainSubstring("packages/app/node_modules"), "the residue path must be reported root-relative, naming the subdirectory setup actually ran in")
	Expect(joined).NotTo(ContainSubstring("unrelated"), "dirt outside WorkingDirectory must never be reported as this run's residue")
	for _, path := range outcome.ChangedPaths {
		Expect(path).NotTo(Equal("packages/"), "the residue disclosure must not collapse to an ancestor directory that doesn't even name node_modules")
	}
}
