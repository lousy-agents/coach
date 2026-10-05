package sourcescope

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeExtendsExtensionlessPathResolves(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":      `{"extends": "./tsconfig.base"}`,
		"tsconfig.base.json": `{"exclude": ["test/**/*.ts"]}`,
		"src/app.ts":         "export const app = 1\n",
		"test/app.ts":        "export const test = 1\n",
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
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts (an extensionless extends target should retry with .json appended)", files)
	}
}

func TestApplySourceScopeExtendsDescendingThenAscendingWithinSnapshotRootSucceeds(t *testing.T) {

	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":              `{"extends": "./packages/foo/tsconfig.json"}`,
		"packages/foo/tsconfig.json": `{"extends": "../../tsconfig.base.json"}`,
		"tsconfig.base.json":         `{"exclude": ["test/**/*.ts"]}`,
		"src/app.ts":                 "export const app = 1\n",
		"test/app.ts":                "export const test = 1\n",
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
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts (the snapshot-root base's exclude should apply even though the chain descends then ascends)", files)
	}
}
