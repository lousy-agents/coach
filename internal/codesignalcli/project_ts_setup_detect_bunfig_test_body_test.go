package codesignalcli

import (
	"os"
	"path/filepath"
	"testing"
)

func body_projectTsSetupDetectBunfigTest_35(t *testing.T, tc struct {
	name     string
	contents string
	want     string
}) {
	root := t.TempDir()
	if tc.contents != "" {
		if err := os.WriteFile(filepath.Join(root, "bunfig.toml"), []byte(tc.contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := detectBunfigHazard(root); got != tc.want {
		t.Fatalf("detectBunfigHazard() = %q, want %q", got, tc.want)
	}
}
