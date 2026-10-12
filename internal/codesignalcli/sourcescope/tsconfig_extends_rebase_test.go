package sourcescope

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
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

	files, _, err := Apply(repo, head, "", "production", []gitrepo.SelectedFile{
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

func TestLoadTSConfigRebasesExtendsPatternsWhenDirIsASymlink(t *testing.T) {

	real := t.TempDir()
	writeScopeTestFile(t, real, "tsconfig.json", `{"extends": "./packages/shared/tsconfig.json"}`)
	writeScopeTestFile(t, real, "packages/shared/tsconfig.json", `{"exclude": ["test/**/*.ts"]}`)

	linked := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(real, linked); err != nil {
		t.Fatal(err)
	}

	config, ok, err := loadTSConfig(linked)
	if err != nil {
		t.Fatalf("loadTSConfig() error = %v", err)
	}
	if !ok {
		t.Fatal("loadTSConfig() ok = false, want true")
	}
	want := []string{"packages/shared/test/**/*.ts"}
	if !reflect.DeepEqual(config.Exclude, want) {
		t.Fatalf("loadTSConfig() Exclude = %v, want %v (the base's exclude pattern must be rebased relative to the resolved snapshot root, not the raw symlinked dir)", config.Exclude, want)
	}
}
