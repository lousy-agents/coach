package sourcescope

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeTreatsUnterminatedBlockCommentAsUnknown(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"exclude": ["test/**/*.ts"]} /* unterminated`,
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
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when tsconfig.json has an unterminated block comment", files)
	}
	for _, file := range files {
		if file.SourceScope != Unknown {
			t.Errorf("%s source scope = %q, want %q", file.Path, file.SourceScope, Unknown)
		}
	}
}

func TestApplySourceScopeToleratesTSConfigComments(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": "{\n" +
			"  // exclude test files from production reports\n" +
			"  \"exclude\": [\"test/**/*.ts\"], /* trailing block comment */\n" +
			"}\n",
		"src/app.ts":  "export const app = 1\n",
		"test/app.ts": "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := Apply(repo, head, "", "production", []gitrepo.SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != Production {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts", files)
	}
}

func TestApplySourceScopeToleratesTSConfigTrailingComma(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"exclude": ["test/**/*.ts",],}`,
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
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != Production {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts", files)
	}
}

func TestApplySourceScopePreservesCommentMarkersInsideStrings(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": "{\n" +
			"  // a real line comment\n" +
			"  \"exclude\": [\"test/**/*.ts\"],\n" +
			"  \"compilerOptions\": {\n" +
			"    \"baseUrl\": \"https://example.com/* not a real comment */path//trailing\"\n" +
			"  }\n" +
			"}\n",
		"src/app.ts":  "export const app = 1\n",
		"test/app.ts": "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := Apply(repo, head, "", "production", []gitrepo.SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != Production {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts", files)
	}
}
