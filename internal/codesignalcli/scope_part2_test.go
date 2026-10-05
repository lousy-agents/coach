package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeRebasesInheritedExcludeToBaseDirectory(t *testing.T) {

	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":                   `{"extends": "./packages/shared/tsconfig.json"}`,
		"packages/shared/tsconfig.json":   `{"exclude": ["test/**/*.ts"]}`,
		"src/app.ts":                      "export const app = 1\n",
		"packages/shared/test/fixture.ts": "export const fixture = 1\n",
		"test/root.ts":                    "export const root = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "packages/shared/test/fixture.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/root.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	paths := map[string]bool{}
	for _, file := range files {
		paths[file.Path] = true
	}
	if !paths["src/app.ts"] {
		t.Errorf("src/app.ts should remain production")
	}
	if !paths["test/root.ts"] {
		t.Errorf("test/root.ts should remain production: the base's exclude, rebased to its own directory (packages/shared/), must not reach a root-level file merely sharing the pattern's relative suffix")
	}
	if paths["packages/shared/test/fixture.ts"] {
		t.Errorf("packages/shared/test/fixture.ts should not be production: the base's exclude, rebased to its own directory, must reach it")
	}
	if len(files) != 2 {
		t.Fatalf("ApplySourceScope() = %#v, want exactly src/app.ts and test/root.ts kept", files)
	}
}

func TestApplySourceScopeTreatsUnterminatedBlockCommentAsUnknown(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"exclude": ["test/**/*.ts"]} /* unterminated`,
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
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when tsconfig.json has an unterminated block comment", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q", file.Path, file.SourceScope, SourceScopeUnknown)
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
