package codesignalcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type sigTestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun struct {
	installDir string
}

func (sigRecv *sigTestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun) call(context.Context, string) (string, bool) {
	pkgDir := filepath.Join(sigRecv.installDir, "node_modules", "typescript")
	if _, statErr := os.Stat(filepath.Join(pkgDir, "package.json")); statErr != nil {
		return "", false
	}
	return pkgDir, true
}

type sigTestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun0 struct {
	installDir string
	t          *testing.
			T
}

func (sigRecv *sigTestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun0) call() {

	if err := os.RemoveAll(sigRecv.installDir); err != nil {
		sigRecv.t.
			Fatalf("remove placeholder install dir: %v", err)
	}
}

type sigTestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun1 struct {
	repo string
	t    *testing.
		T
}

func (sigRecv *sigTestRunPrepareCompilerMiseSetupInstallsAndReflectsOnRerun1) call() {

	if err := os.WriteFile(filepath.Join(sigRecv.repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644); err != nil {
		sigRecv.t.
			Fatalf("write mise.toml: %v", err)
	}
}
