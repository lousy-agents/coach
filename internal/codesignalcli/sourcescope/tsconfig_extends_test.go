package sourcescope

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeAppliesExtendedBaseTSConfig(t *testing.T) {

	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":      `{"extends": "./base/tsconfig.json"}`,
		"base/tsconfig.json": `{"exclude": ["test/**/*.ts"]}`,
		"src/app.ts":         "export const app = 1\n",
		"base/test/app.ts":   "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := Apply(repo, head, "", "production", []gitrepo.SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "base/test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != Production {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts (base config's exclude, rebased to its own directory, should apply)", files)
	}
}

func TestApplySourceScopeChildTSConfigOverridesExtendedBaseInclude(t *testing.T) {

	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":                `{"extends": "./base/tsconfig.json", "include": ["src/**/*.ts", "base/**/*.ts"]}`,
		"base/tsconfig.json":           `{"include": ["other/**/*.ts"], "exclude": ["src/excluded/**/*.ts"]}`,
		"src/app.ts":                   "export const app = 1\n",
		"other/app.ts":                 "export const other = 1\n",
		"base/src/excluded/fixture.ts": "export const fixture = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := Apply(repo, head, "", "production", []gitrepo.SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "other/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "base/src/excluded/fixture.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != Production {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts: "+
			"child's own include must override (not merge with) the base's include (other/app.ts), "+
			"while the base's exclude must still apply (rebased to its own directory) since the child omits its own (base/src/excluded/fixture.ts)", files)
	}
}

func TestApplySourceScopeAppliesTwoLevelExtendedBaseTSConfig(t *testing.T) {

	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":          `{"extends": "./mid/tsconfig.json"}`,
		"mid/tsconfig.json":      `{"extends": "./root/tsconfig.json"}`,
		"mid/root/tsconfig.json": `{"exclude": ["test/**/*.ts"]}`,
		"src/app.ts":             "export const app = 1\n",
		"mid/root/test/app.ts":   "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := Apply(repo, head, "", "production", []gitrepo.SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "mid/root/test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != Production {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts (root base's exclude, rebased to its own directory, should apply through a two-level extends chain)", files)
	}
}

func TestApplySourceScopeArrayExtendsPreservesChildOwnSettings(t *testing.T) {

	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"extends": ["./a.json", "./b.json"], "exclude": ["test/**/*.ts"]}`,
		"a.json":        `{}`,
		"b.json":        `{}`,
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
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts: an array-valued (multi-base) extends should be treated as absent, "+
			"not discard the child's own exclude", files)
	}
}
