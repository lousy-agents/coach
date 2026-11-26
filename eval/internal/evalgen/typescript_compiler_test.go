package evalgen_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

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
