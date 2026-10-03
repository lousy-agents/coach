package codesignalcli

import (
	"os"

	"path/filepath"

	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeArrayExtendsPreservesChildOwnSettings(t *testing.T) {

	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"extends": ["./a.json", "./b.json"], "exclude": ["test/**/*.ts"]}`,
		"a.json":        `{}`,
		"b.json":        `{}`,
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
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != SourceScopeProduction {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts: an array-valued (multi-base) extends should be treated as absent, "+
			"not discard the child's own exclude", files)
	}
}

func TestApplySourceScopeExtendsExtensionlessPathResolves(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":      `{"extends": "./tsconfig.base"}`,
		"tsconfig.base.json": `{"exclude": ["test/**/*.ts"]}`,
		"src/app.ts":         "export const app = 1\n",
		"test/app.ts":        "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != SourceScopeProduction {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts (an extensionless extends target should retry with .json appended)", files)
	}
}

func TestApplySourceScopeExcludesGoTestFilesWithoutBuildTarget(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"shipping/shipping.go":      "package shipping\n\nfunc Update() {}\n",
		"shipping/shipping_test.go": "package shipping\n\nfunc TestUpdate() {}\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "shipping/shipping.go", Status: "modified", Language: semantics.LanguageGo},
		{Path: "shipping/shipping_test.go", Status: "modified", Language: semantics.LanguageGo},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "shipping/shipping.go" || files[0].SourceScope != SourceScopeUnknown {
		t.Fatalf("production scope without a build target should retain only non-test Go files as unknown: got %#v", files)
	}
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
