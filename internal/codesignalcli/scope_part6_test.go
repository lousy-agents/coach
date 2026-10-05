package codesignalcli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeTSConfigExtendsAbsolutePathOutsideSnapshotFailsOpen(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"extends": "/etc/passwd"}`,
		"src/app.ts":    "export const app = 1\n",
		"test/app.ts":   "export const test = 1\n",
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
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when extends is an absolute path outside the snapshot", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q (extends outside the snapshot must fail open, same as no tsconfig.json)", file.Path, file.SourceScope, SourceScopeUnknown)
		}
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

func TestApplyBaselineSourceScopeAllReturnsEverythingUnexcluded(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"shipping/shipping.go":      "package shipping\n\nfunc Update() {}\n",
		"shipping/shipping_test.go": "package shipping\n\nfunc TestUpdate() {}\n",
	})
	head := scopeTestCommit(t, repo)

	kept, excluded, err := ApplyBaselineSourceScope(repo, head, "", "all", []gitrepo.SelectedFile{
		{Path: "shipping/shipping.go", Language: semantics.LanguageGo},
		{Path: "shipping/shipping_test.go", Language: semantics.LanguageGo},
	})
	if err != nil {
		t.Fatalf("ApplyBaselineSourceScope() error = %v", err)
	}

	if len(kept) != 2 {
		t.Fatalf("ApplyBaselineSourceScope() kept = %#v, want both files for scope=all", kept)
	}
	if len(excluded) != 0 {
		t.Fatalf("ApplyBaselineSourceScope() excluded = %#v, want empty for scope=all", excluded)
	}
}
