package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeTSConfigExtendsBareNpmSpecifierFailsOpen(t *testing.T) {

	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":    `{"extends": "some-base-config"}`,
		"some-base-config": `{"files": ["src/app.ts"]}`,
		"src/app.ts":       "export const app = 1\n",
		"test/app.ts":      "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := ApplySourceScope(repo, head, "", "production", []gitrepo.SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when extends is a bare, unscoped npm-style package specifier", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q (a bare npm-package extends target must fail open, same as no tsconfig.json)", file.Path, file.SourceScope, SourceScopeUnknown)
		}
	}
}

func TestApplySourceScopeTSConfigExtendsChainHittingNpmSpecifierMidChainFailsOpen(t *testing.T) {

	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":                      `{"extends": "./sub/tsconfig.json"}`,
		"sub/tsconfig.json":                  `{"extends": "@tsconfig/node18/tsconfig.json"}`,
		"sub/@tsconfig/node18/tsconfig.json": `{"files": ["src/app.ts"]}`,
		"src/app.ts":                         "export const app = 1\n",
		"test/app.ts":                        "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := ApplySourceScope(repo, head, "", "production", []gitrepo.SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when an npm-package specifier appears mid-chain", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q (an npm-package extends target mid-chain must fail the whole chain open)", file.Path, file.SourceScope, SourceScopeUnknown)
		}
	}
}

func TestApplyBaselineSourceScopeTalliesExcludedFiles(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"shipping/shipping.go":      "package shipping\n\nfunc Update() {}\n",
		"shipping/shipping_test.go": "package shipping\n\nfunc TestUpdate() {}\n",
	})
	head := scopeTestCommit(t, repo)

	kept, excluded, err := ApplyBaselineSourceScope(repo, head, "", "production", []gitrepo.SelectedFile{
		{Path: "shipping/shipping.go", Language: semantics.LanguageGo},
		{Path: "shipping/shipping_test.go", Language: semantics.LanguageGo},
	})
	if err != nil {
		t.Fatalf("ApplyBaselineSourceScope() error = %v", err)
	}

	if len(kept) != 1 || kept[0].Path != "shipping/shipping.go" || kept[0].SourceScope != SourceScopeUnknown {
		t.Fatalf("ApplyBaselineSourceScope() kept = %#v, want only shipping.go", kept)
	}

	if len(excluded) != 1 || excluded[0].Reason != SourceScopeTestOnly || excluded[0].Language != string(semantics.LanguageGo) || excluded[0].Count != 1 {
		t.Fatalf("ApplyBaselineSourceScope() excluded = %#v, want one test_only/go group of count 1", excluded)
	}
}
