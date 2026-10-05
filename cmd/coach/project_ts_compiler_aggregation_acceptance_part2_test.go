package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// writeVersionedStubMiseScript extends writeStubMiseScript's contract
// (project_readiness_acceptance_test.go) with distinct, independently
// controllable responses for `mise --version` and `mise config ls -J`.
// That shared helper cannot do this itself: it echoes the same miseVersion
// argument for every invocation except `where`, which represents a
// TypeScript version in every spec that already depends on it, not a
// mise-tool version -- reusing it here would make every existing mise-origin
// spec in this file collide with the new mise-tool-version gate.
//
// tsVersion seeds the `where`-fixture on disk (a real installed compiler at
// that version) but is otherwise unused unless whereVersion is requested;
// globalConfigVersion is what `config get tools.npm:typescript -g` reports
// (empty means "not configured", matching detectGlobalMiseTypescriptVersion's
// contract); toolVersionOutput is what `--version` reports (empty means the
// probe fails, modeling an undetectable mise-tool version); configLsJSON is
// the raw `config ls -J` response.
func writeVersionedStubMiseScript(tsVersion, toolVersionOutput, configLsJSON, globalConfigVersion string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubmise-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	installDir := filepath.Join(dir, "install")
	if tsVersion != "" {
		Expect(os.MkdirAll(filepath.Join(installDir, "node_modules", "typescript"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(installDir, "node_modules", "typescript", "package.json"), []byte(fmt.Sprintf(`{"name":"typescript","version":%q}`+"\n", tsVersion)), 0o644)).To(Succeed())
		nativeUnscoped := fmt.Sprintf("typescript-%s-%s", runtime.GOOS, npmArchName())
		nativeDir := filepath.Join(installDir, "node_modules", "@typescript", nativeUnscoped)
		Expect(os.MkdirAll(nativeDir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(nativeDir, "package.json"), []byte(fmt.Sprintf(`{"name":%q,"version":%q}`+"\n", "@typescript/"+nativeUnscoped, tsVersion)), 0o644)).To(Succeed())
	}

	versionBranch := "exit 1"
	if toolVersionOutput != "" {
		versionBranch = fmt.Sprintf("echo %q; exit 0", toolVersionOutput)
	}
	configGetBranch := "exit 1"
	if globalConfigVersion != "" {
		configGetBranch = fmt.Sprintf("echo %q; exit 0", globalConfigVersion)
	}
	script := fmt.Sprintf(
		"#!/bin/sh\necho \"$PWD\" >> %q\necho \"$@\" >> %q\n"+
			"if [ \"$1\" = \"--version\" ]; then %s; fi\n"+
			"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo %q; exit 0; fi\n"+
			"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"get\" ]; then %s; fi\n"+
			"if [ \"$1\" = \"where\" ]; then echo %q; exit 0; fi\n"+
			"exit 1\n",
		filepath.Join(dir, stubMiseCwdLog), filepath.Join(dir, stubMiseInvocationLog),
		versionBranch, configLsJSON, configGetBranch, installDir,
	)
	Expect(os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755)).To(Succeed())
	return dir
}

func declarationMismatchWarning(doc readinessResultDoc) (declared, found, origin string, present bool) {
	for _, warning := range doc.Warnings {
		if warning.Code == "compiler_declaration_mismatch" {
			return warning.DeclaredVersion, warning.FoundVersion, warning.DeclarationOrigin, true
		}
	}
	return "", "", "", false
}

func rootFindingPairs(doc readinessResultDoc) []string {
	pairs := make([]string, 0, len(doc.Checks.Compiler.RootFindings))
	for _, finding := range doc.Checks.Compiler.RootFindings {
		if finding.Version == "" {
			pairs = append(pairs, finding.Root)
			continue
		}
		pairs = append(pairs, finding.Root+"@"+finding.Version)
	}
	return pairs
}

func gapEntries(doc readinessResultDoc) []string {
	entries := make([]string, len(doc.Gaps))
	for i, g := range doc.Gaps {
		if g.PackageManagerKind == "" {
			entries[i] = g.Code
			continue
		}
		entries[i] = g.Code + ":" + g.PackageManagerKind
	}
	return entries
}

func prepareCompilerChoices(doc readinessResultDoc) ([]string, bool) {
	for _, action := range doc.NextActions {
		if action.Kind == "prepare_compiler" {
			return action.Choices, true
		}
	}
	return nil, false
}
