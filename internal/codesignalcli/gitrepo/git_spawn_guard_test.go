package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every git child of this package is built by gitCommand, which is where its environment is hardened.
// A runner that built its own would start git with the caller's environment and skip that.
func TestOnlyGitCommandStartsAGitProcess(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == "git_command.go" {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(source), "exec.Command") {
			t.Errorf("%s starts a process itself; build the git child with gitCommand so it keeps the hardened environment", file)
		}
	}
}
