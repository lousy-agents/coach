package evalgen_test

import (
	"os"
	"path/filepath"
	"strings"
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

func TestAnEvalTSCNamingTheTypeScriptPackageDirectoryIsTheEntryPointInsideIt(t *testing.T) {
	root := repositoryRoot(t)
	packageDir := filepath.Join(root, "js", "semantics", "node_modules", "typescript")
	t.Setenv("EVAL_TSC", packageDir)

	tools, err := evalgen.ResolveTypeScriptTools(root)

	if err != nil {
		t.Fatalf("the package directory is refused: %v (run `mise run js-install` if the compiler is missing)", err)
	}
	if want := filepath.Join(packageDir, "bin", "tsc"); tools.TSC != want {
		t.Fatalf("the compiler is %q, want the entry point %q", tools.TSC, want)
	}
}

func TestAnEvalTSCThatIsADirectoryWithoutAnEntryPointSaysWhatItExpected(t *testing.T) {
	t.Setenv("EVAL_TSC", t.TempDir())

	_, err := evalgen.ResolveTypeScriptTools(repositoryRoot(t))

	expectRefusal(t, err, "expected the tsc entry point")
}

func TestAnEvalTSCThatPrintsNoVersionSaysWhatItExpected(t *testing.T) {
	t.Setenv("EVAL_TSC", writeScript(t, t.TempDir(), "tsc", "exit 0"))

	_, err := evalgen.ResolveTypeScriptTools(repositoryRoot(t))

	expectRefusal(t, err, "expected the tsc entry point")
}

func TestAWrongVersionIsStillNamedByItsVersionNotAsAMissingEntryPoint(t *testing.T) {
	t.Setenv("EVAL_TSC", writeScript(t, t.TempDir(), "tsc", "echo 'Version 5.9.2'"))

	_, err := evalgen.ResolveTypeScriptTools(repositoryRoot(t))

	expectRefusal(t, err, "5.9.2")
	if strings.Contains(err.Error(), "entry point") {
		t.Fatalf("a real compiler of the wrong version is not a missing entry point: %v", err)
	}
}
