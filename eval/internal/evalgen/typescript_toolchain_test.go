package evalgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func expectRefusal(t *testing.T, err error, words ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a refusal naming %v", words)
	}
	for _, word := range words {
		if !strings.Contains(err.Error(), word) {
			t.Fatalf("refusal %q does not name %q", err, word)
		}
	}
}

func TestACheckoutWithoutTheInstalledCompilerNamesTheInstallTask(t *testing.T) {
	t.Setenv("EVAL_TSC", "")
	checkout := t.TempDir()
	if err := os.MkdirAll(filepath.Join(checkout, "eval"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "eval", "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := evalgen.ResolveTypeScriptTools(checkout)

	expectRefusal(t, err, "js-install")
}

func TestACompilerThatReportsTheVersionButFailsIsRefused(t *testing.T) {
	t.Setenv("EVAL_TSC", writeScript(t, t.TempDir(), "tsc", "echo 'Version "+evalgen.PinnedTypeScript+"'\nexit 1"))

	_, err := evalgen.ResolveTypeScriptTools(repositoryRoot(t))

	expectRefusal(t, err, evalgen.PinnedTypeScript)
}

func TestTheInstalledCompilerIsFoundThroughASymbolicLinkIntoTheCheckout(t *testing.T) {
	t.Setenv("EVAL_TSC", "")
	link := filepath.Join(t.TempDir(), "cmd")
	if err := os.Symlink(filepath.Join(repositoryRoot(t), "eval", "cmd"), link); err != nil {
		t.Fatal(err)
	}

	tools, err := evalgen.ResolveTypeScriptTools(link)

	if err != nil {
		t.Fatalf("a link into the checkout is refused: %v (run `mise run js-install` if the compiler is missing)", err)
	}
	if !filepath.IsAbs(tools.TSC) {
		t.Fatalf("the compiler %q is not an absolute path", tools.TSC)
	}
}

func TestANodeBelowTheSupportedMajorIsRefused(t *testing.T) {
	stub := t.TempDir()
	writeScript(t, stub, "node", "echo v22.1.0")
	t.Setenv("PATH", stub+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := evalgen.ResolveTypeScriptTools(repositoryRoot(t))

	expectRefusal(t, err, "node", "24", "v22.1.0")
}

func TestARelativeEvalTSCBecomesAbsolute(t *testing.T) {
	root := repositoryRoot(t)
	t.Chdir(filepath.Join(root, "js", "semantics"))
	t.Setenv("EVAL_TSC", filepath.Join("node_modules", ".bin", "tsc"))

	tools, err := evalgen.ResolveTypeScriptTools(root)

	if err != nil {
		t.Fatalf("a relative EVAL_TSC is refused: %v (run `mise run js-install` if the compiler is missing)", err)
	}
	if !filepath.IsAbs(tools.TSC) {
		t.Fatalf("the compiler %q is not an absolute path, so a validation run in another directory cannot start it", tools.TSC)
	}
}
