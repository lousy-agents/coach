package codesignalcli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeUsesHEADSnapshotAndTSConfigFiles(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"files":["src/app.ts"]}`,
		"src/app.ts":    "export const app = 1\n",
		"test/app.ts":   "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	// A live config must not alter the report for the committed range.
	writeScopeTestFile(t, repo, "tsconfig.json", "not valid json")
	files, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != SourceScopeProduction {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts", files)
	}
}

func TestTSConfigExplicitEmptyFilesSelectsNoFiles(t *testing.T) {
	emptyFiles := []string{}
	config := tsConfig{Files: &emptyFiles}
	if config.matchesInclude("src/app.ts") {
		t.Fatal("matchesInclude() = true, want false for an explicit empty files setting")
	}
}

func TestApplySourceScopeResolvesGoTargetFromInvocationSubdirectory(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"go.mod":              "module example.com/scope-test\n\ngo 1.24\n",
		"cmd/app/main.go":     "package main\n\nimport \"example.com/scope-test/internal/app\"\n\nfunc main() { app.Run() }\n",
		"internal/app/app.go": "package app\n\nfunc Run() {}\n",
	})
	head := scopeTestCommit(t, repo)

	files, err := ApplySourceScope(filepath.Join(repo, "cmd", "app"), head, ".", "production", []SelectedFile{
		{Path: "cmd/app/main.go", Status: "modified", Language: semantics.LanguageGo},
		{Path: "internal/app/app.go", Status: "modified", Language: semantics.LanguageGo},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ApplySourceScope() kept %d files, want 2: %#v", len(files), files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeProduction {
			t.Errorf("%s source scope = %q, want %q", file.Path, file.SourceScope, SourceScopeProduction)
		}
	}
}

func newScopeTestRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	runScopeTestCommand(t, repo, "git", "init")
	for path, content := range files {
		writeScopeTestFile(t, repo, path, content)
	}
	return repo
}

func writeScopeTestFile(t *testing.T, repo, path, content string) {
	t.Helper()
	filename := filepath.Join(repo, path)
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scopeTestCommit(t *testing.T, repo string) string {
	t.Helper()
	runScopeTestCommand(t, repo, "git", "add", ".")
	runScopeTestCommand(t, repo, "git", "-c", "user.name=scope-test", "-c", "user.email=scope-test@example.com", "commit", "-m", "fixture")
	return strings.TrimSpace(runScopeTestCommand(t, repo, "git", "rev-parse", "HEAD"))
}

func runScopeTestCommand(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
	return string(output)
}
