package codesignalcli

import (
	"os"

	"path/filepath"

	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeTSConfigExtendsSymlinkEscapingSnapshotFailsOpen(t *testing.T) {

	outside := t.TempDir()
	secretPath := filepath.Join(outside, "secret.json")
	if err := os.WriteFile(secretPath, []byte(`{"files": ["src/app.ts"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"extends": "./base.json"}`,
		"src/app.ts":    "export const app = 1\n",
		"test/app.ts":   "export const test = 1\n",
	})
	if err := os.Symlink(secretPath, filepath.Join(repo, "base.json")); err != nil {
		t.Fatal(err)
	}
	head := scopeTestCommit(t, repo)

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when the extends target is a symlink escaping the snapshot", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q (a symlinked extends target escaping the snapshot must fail open rather than reading through it)", file.Path, file.SourceScope, SourceScopeUnknown)
		}
	}
}

func TestApplySourceScopeTreatsGenuinelyInvalidTSConfigAsUnknown(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": "not valid json at all {{{",
		"src/app.ts":    "export const app = 1\n",
		"test/app.ts":   "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when tsconfig.json is genuinely invalid", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q", file.Path, file.SourceScope, SourceScopeUnknown)
		}
	}
}

func TestTSConfigExplicitEmptyFilesSelectsNoFiles(t *testing.T) {
	emptyFiles := []string{}
	config := tsConfig{Files: &emptyFiles}
	if config.matchesInclude("src/app.ts") {
		t.Fatal("matchesInclude() = true, want false for an explicit empty files setting")
	}
}

func scopeTestCommit(t *testing.T, repo string) string {
	t.Helper()
	runScopeTestCommand(t, repo, "git", "add", ".")
	runScopeTestCommand(t, repo, "git", "-c", "user.name=scope-test", "-c", "user.email=scope-test@example.com", "commit", "-m", "fixture")
	return strings.TrimSpace(runScopeTestCommand(t, repo, "git", "rev-parse", "HEAD"))
}
