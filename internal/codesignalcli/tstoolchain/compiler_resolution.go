// Package tstoolchain resolves the host toolchain a TypeScript project scan
// runs on: the supported Node runtime and the one approved TypeScript
// compiler, found across the project manifest, project mise, and global mise
// origins, with mise's own trust gates applied before mise is ever asked.
package tstoolchain

import (
	"strings"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const (
	OriginProject             = "project"
	OriginMiseProject         = "mise_project"
	OriginMiseGlobal          = "mise_global"
	DeclarationOriginManifest = "manifest"
)

func ResolveCompiler(dir string, roots []string) projectreadiness.Check {
	return compilerCheckFromAggregate(EvaluateOrigins(dir, roots))
}

const (
	compilerWorktreeRootTimeout   = 10 * time.Second
	maxCompilerWorktreeRootOutput = 4 << 10
	maxCompilerWorktreeRootStderr = 4 << 10
)

// WorktreeRoot is the walk ceiling for nearest-package.json
// resolution, not the project-manifest origin. projectcheck.Run has
// already failed closed on an unreadable worktree by the time this runs, so
// a resolution failure here falls back to dir rather than blocking the
// compiler check.
func WorktreeRoot(dir string) string {
	output, err := gitrepo.RunBytesBounded(dir, maxCompilerWorktreeRootOutput, maxCompilerWorktreeRootStderr, compilerWorktreeRootTimeout, "rev-parse", "--show-toplevel")
	if err != nil {
		return dir
	}
	root := strings.TrimSpace(string(output))
	if root == "" {
		return dir
	}
	return root
}

type CompilerResolution struct {
	Origin            string
	Version           string
	Path              string
	NativePackagePath string
}

func compilerUnresolved(code string) *CompilerUnresolvedError {
	return &CompilerUnresolvedError{Code: code}
}

func ResolveCompilerForRuntime(dir string, roots []string) (CompilerResolution, error) {
	aggregate := EvaluateOrigins(dir, roots)
	code, findings := compilerOutcomeFromAggregate(aggregate)
	if code != "" {
		return CompilerResolution{}, &CompilerUnresolvedError{Code: code, RootFindings: findings}
	}
	return CompilerResolution{
		Origin:            aggregate.Winner.Origin,
		Version:           aggregate.Winner.Version,
		Path:              aggregate.Winner.Path,
		NativePackagePath: aggregate.Winner.NativePath,
	}, nil
}
