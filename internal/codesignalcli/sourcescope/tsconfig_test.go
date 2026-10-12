package sourcescope

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeIncludeAndFilesAreAdditive(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":         `{"files": ["src/explicit.ts"], "include": ["src/included/**/*.ts"]}`,
		"src/explicit.ts":       "export const explicit = 1\n",
		"src/included/extra.ts": "export const extra = 1\n",
		"src/other.ts":          "export const other = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := Apply(repo, head, "", "production", []gitrepo.SelectedFile{
		{Path: "src/explicit.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "src/included/extra.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "src/other.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	paths := map[string]bool{}
	for _, file := range files {
		paths[file.Path] = true
	}
	if !paths["src/explicit.ts"] {
		t.Errorf("src/explicit.ts should be production via the explicit files entry")
	}
	if !paths["src/included/extra.ts"] {
		t.Errorf("src/included/extra.ts should be production via the include pattern, even though files is also set")
	}
	if paths["src/other.ts"] {
		t.Errorf("src/other.ts should not be production: it matches neither files nor include")
	}
	if len(files) != 2 {
		t.Fatalf("ApplySourceScope() = %#v, want exactly the files+include union", files)
	}
}

func TestApplySourceScopeTreatsGenuinelyInvalidTSConfigAsUnknown(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": "not valid json at all {{{",
		"src/app.ts":    "export const app = 1\n",
		"test/app.ts":   "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := Apply(repo, head, "", "production", []gitrepo.SelectedFile{
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
		if file.SourceScope != Unknown {
			t.Errorf("%s source scope = %q, want %q", file.Path, file.SourceScope, Unknown)
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
