package tssetup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// A policy the user names with --project-config is not one Coach would discover, so a compiler
// setup flow that is handed the zero policyref.Ref instead of the named one looks at a different
// policy than the scan that asked for the setup, or at none. Every flow below is run with a policy
// at a path no discovery finds, so the only way it sees the policy is by being handed it.
const namedPolicyPath = "config/review-policy.json"

// namedPolicyRepo is a repository whose only policy is committed at namedPolicyPath and whose
// mise.toml pins TypeScript, so the project mise scope is an offer that is worth making. It
// returns the repository and its revision.
func namedPolicyRepo(t *testing.T) (repo, revision string) {
	t.Helper()
	repo = gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	revision = gitfixture.CommitFile(t, repo, namedPolicyPath, `{"schema_version":"1","roots":["."]}`+"\n")
	writeProjectMiseTypescriptPin(t, repo, "7.0.2")
	return repo, revision
}

// namedPolicyRepoWithTwoRoots is a repository whose only policy, committed at namedPolicyPath, selects
// two package directories. The two roots are what tell a flow that read the named policy from one that
// read none: with no policy, the roots fall back to the repository root, and the answers differ.
func namedPolicyRepoWithTwoRoots(t *testing.T) (repo, revision string) {
	t.Helper()
	repo = gitfixture.Init(t)
	writePackageTypescript(t, repo, "packages/a", "5.0.0")
	writePackageTypescript(t, repo, "packages/b", "5.0.1")
	writeProjectMiseTypescriptPin(t, repo, "7.0.2")
	revision = gitfixture.CommitFile(t, repo, namedPolicyPath, `{"schema_version":"1","roots":["packages/a","packages/b"]}`+"\n")
	return repo, revision
}

// stubNodeOnPath puts a `node` on PATH that reports version, so the readiness runtime check does not
// read the host's node: the offer is withheld when the runtime is unsupported, and whether the host's
// node is supported is not what these tests are about.
func stubNodeOnPath(t *testing.T, version string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo " + version + "; exit 0; fi\n" +
		"if [ \"$1\" = \"-p\" ]; then echo \"$0\"; exit 0; fi\necho " + version + "\n"
	if err := os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755); err != nil {
		t.Fatalf("write stub node: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// stubMiseThatInstallsTypescript makes `mise install` put the compiler where the post-install
// recheck looks for it, as the existing install test does, and gives the flow a supported node.
func stubMiseThatInstallsTypescript(t *testing.T) {
	t.Helper()
	stubNodeOnPath(t, "v24.9.9")
	installDir := notYetInstalledDir(t)
	writeStatefulStubMiseOnPath(t, installDir, "7.0.2")
	originalLocate := tstoolchain.LocateMiseTypescriptInstall
	t.Cleanup(func() { tstoolchain.LocateMiseTypescriptInstall = originalLocate })
	tstoolchain.LocateMiseTypescriptInstall = locateInstalledTypescriptUnder(installDir)
}

func requirePolicyAndCompilerAfterInstall(t *testing.T, flow string, post *projectreadiness.Result) {
	t.Helper()
	if post == nil {
		t.Fatalf("%s: no post-install readiness was computed", flow)
	}
	if post.Checks.Policy.State != projectreadiness.Pass {
		t.Errorf("%s: post-install policy check = %+v, want pass: the recheck must read the policy the scan was given, not a discovered one", flow, post.Checks.Policy)
	}
	if post.Checks.Compiler.State != projectreadiness.Pass {
		t.Errorf("%s: post-install compiler check = %+v, want pass", flow, post.Checks.Compiler)
	}
}
