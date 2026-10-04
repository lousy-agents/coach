package claudehooks

import (
	"os"

	"path/filepath"

	"testing"
)

func setupTestDirs(t *testing.T) (tmp, home, project, bin, npmDir, localBin string) {
	tmp = t.TempDir()
	home = filepath.Join(tmp, "home")
	project = filepath.Join(tmp, "project")
	bin = filepath.Join(tmp, "bin")
	npmDir = filepath.Join(tmp, "npm")
	localBin = filepath.Join(home, ".local", "bin")
	for _, d := range []string{home, project, bin, npmDir, localBin} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	miseToml := `min_version = "2026.7.7"
[tools]
go = "1.26.5"
node = "24"
`
	if err := os.WriteFile(filepath.Join(project, "mise.toml"), []byte(miseToml), 0644); err != nil {
		t.Fatal(err)
	}
	return
}
