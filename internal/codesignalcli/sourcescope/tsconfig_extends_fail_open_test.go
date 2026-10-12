package sourcescope

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeCircularExtendsChainFailsOpen(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"extends": "./base.json", "exclude": ["test/**/*.ts"]}`,
		"base.json":     `{"extends": "./tsconfig.json"}`,
		"src/app.ts":    "export const app = 1\n",
		"test/app.ts":   "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	expectSourceAndTestFilesFailOpen(t, repo, head, "when the extends chain is circular", "a circular extends chain must fail open, same as no tsconfig.json")
}

func TestApplySourceScopeTSConfigExtendsEscapingSnapshotFailsOpen(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"extends": "../../../../../../etc/passwd"}`,
		"src/app.ts":    "export const app = 1\n",
		"test/app.ts":   "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	expectSourceAndTestFilesFailOpen(t, repo, head, "when extends escapes the snapshot directory", "extends escaping the snapshot must fail open, same as no tsconfig.json")
}

func TestApplySourceScopeTSConfigExtendsScopedNpmSpecifierFailsOpen(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":                  `{"extends": "@tsconfig/node18/tsconfig.json"}`,
		"@tsconfig/node18/tsconfig.json": `{"files": ["src/app.ts"]}`,
		"src/app.ts":                     "export const app = 1\n",
		"test/app.ts":                    "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	expectSourceAndTestFilesFailOpen(t, repo, head, "when extends is a scoped npm package specifier", "a scoped npm-package extends target must fail open, same as no tsconfig.json")
}

func TestApplySourceScopeTSConfigExtendsBareNpmSpecifierFailsOpen(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":    `{"extends": "some-base-config"}`,
		"some-base-config": `{"files": ["src/app.ts"]}`,
		"src/app.ts":       "export const app = 1\n",
		"test/app.ts":      "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	expectSourceAndTestFilesFailOpen(t, repo, head, "when extends is a bare, unscoped npm-style package specifier", "a bare npm-package extends target must fail open, same as no tsconfig.json")
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

	expectSourceAndTestFilesFailOpen(t, repo, head, "when an npm-package specifier appears mid-chain", "an npm-package extends target mid-chain must fail the whole chain open")
}

func TestApplySourceScopeTSConfigExtendsAbsolutePathOutsideSnapshotFailsOpen(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"extends": "/etc/passwd"}`,
		"src/app.ts":    "export const app = 1\n",
		"test/app.ts":   "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	expectSourceAndTestFilesFailOpen(t, repo, head, "when extends is an absolute path outside the snapshot", "extends outside the snapshot must fail open, same as no tsconfig.json")
}

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

	expectSourceAndTestFilesFailOpen(t, repo, head, "when the extends target is a symlink escaping the snapshot", "a symlinked extends target escaping the snapshot must fail open rather than reading through it")
}

// expectSourceAndTestFilesFailOpen applies production scope to src/app.ts and
// test/app.ts at head and asserts both are retained as unknown: a tsconfig
// that cannot be trusted classifies nothing, exactly as if it were absent.
func expectSourceAndTestFilesFailOpen(t *testing.T, repo, head, when, rule string) {
	t.Helper()
	files, _, err := Apply(repo, head, "", "production", []gitrepo.SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown %s", files, when)
	}
	for _, file := range files {
		if file.SourceScope != Unknown {
			t.Errorf("%s source scope = %q, want %q (%s)", file.Path, file.SourceScope, Unknown, rule)
		}
	}
}
