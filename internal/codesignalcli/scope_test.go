package codesignalcli

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestApplySourceScopeUsesHEADSnapshotAndTSConfigFiles(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"files":["src/app.ts"]}`,
		"src/app.ts":    "export const app = 1\n",
		"test/app.ts":   "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	// A live config must not alter the report for the committed range.
	writeScopeTestFile(t, repo, "tsconfig.json", "not valid json")
	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != SourceScopeProduction {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts", files)
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

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != SourceScopeProduction {
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

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != SourceScopeProduction {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts", files)
	}
}

func TestApplySourceScopeTreatsGenuinelyInvalidTSConfigAsUnknown(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": "not valid json at all {{{",
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
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when tsconfig.json is genuinely invalid", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q", file.Path, file.SourceScope, SourceScopeUnknown)
		}
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

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != SourceScopeProduction {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts", files)
	}
}

func TestApplySourceScopeAppliesExtendedBaseTSConfig(t *testing.T) {
	// Exclude is relative to base/, so the hit must live under base/.
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":      `{"extends": "./base/tsconfig.json"}`,
		"base/tsconfig.json": `{"exclude": ["test/**/*.ts"]}`,
		"src/app.ts":         "export const app = 1\n",
		"base/test/app.ts":   "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "base/test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != SourceScopeProduction {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts (base config's exclude, rebased to its own directory, should apply)", files)
	}
}

func TestApplySourceScopeChildTSConfigOverridesExtendedBaseInclude(t *testing.T) {
	// Child include replaces base include; base exclude is still inherited (rebased).
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":                `{"extends": "./base/tsconfig.json", "include": ["src/**/*.ts", "base/**/*.ts"]}`,
		"base/tsconfig.json":           `{"include": ["other/**/*.ts"], "exclude": ["src/excluded/**/*.ts"]}`,
		"src/app.ts":                   "export const app = 1\n",
		"other/app.ts":                 "export const other = 1\n",
		"base/src/excluded/fixture.ts": "export const fixture = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "other/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "base/src/excluded/fixture.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != SourceScopeProduction {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts: "+
			"child's own include must override (not merge with) the base's include (other/app.ts), "+
			"while the base's exclude must still apply (rebased to its own directory) since the child omits its own (base/src/excluded/fixture.ts)", files)
	}
}

func TestApplySourceScopeAppliesTwoLevelExtendedBaseTSConfig(t *testing.T) {
	// Outermost base exclude must apply through the chain; hit under mid/root/.
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":          `{"extends": "./mid/tsconfig.json"}`,
		"mid/tsconfig.json":      `{"extends": "./root/tsconfig.json"}`,
		"mid/root/tsconfig.json": `{"exclude": ["test/**/*.ts"]}`,
		"src/app.ts":             "export const app = 1\n",
		"mid/root/test/app.ts":   "export const test = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "mid/root/test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 1 || files[0].Path != "src/app.ts" || files[0].SourceScope != SourceScopeProduction {
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts (root base's exclude, rebased to its own directory, should apply through a two-level extends chain)", files)
	}
}

func TestApplySourceScopeExtendsDescendingThenAscendingWithinSnapshotRootSucceeds(t *testing.T) {
	// Boundary is snapshot root, not the current hop dir (down then up stays in-bounds).
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":              `{"extends": "./packages/foo/tsconfig.json"}`,
		"packages/foo/tsconfig.json": `{"extends": "../../tsconfig.base.json"}`,
		"tsconfig.base.json":         `{"exclude": ["test/**/*.ts"]}`,
		"src/app.ts":                 "export const app = 1\n",
		"test/app.ts":                "export const test = 1\n",
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
		t.Fatalf("ApplySourceScope() = %#v, want only production src/app.ts (the snapshot-root base's exclude should apply even though the chain descends then ascends)", files)
	}
}

func TestApplySourceScopeCircularExtendsChainFailsOpen(t *testing.T) {
	// In-bounds cycle: only cycle detection (not the path boundary) stops this.
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"extends": "./base.json", "exclude": ["test/**/*.ts"]}`,
		"base.json":     `{"extends": "./tsconfig.json"}`,
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
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when the extends chain is circular", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q (a circular extends chain must fail open, same as no tsconfig.json)", file.Path, file.SourceScope, SourceScopeUnknown)
		}
	}
}

func TestApplySourceScopeTSConfigExtendsEscapingSnapshotFailsOpen(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"extends": "../../../../../../etc/passwd"}`,
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
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when extends escapes the snapshot directory", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q (extends escaping the snapshot must fail open, same as no tsconfig.json)", file.Path, file.SourceScope, SourceScopeUnknown)
		}
	}
}

func TestApplySourceScopeTSConfigExtendsScopedNpmSpecifierFailsOpen(t *testing.T) {
	// Plant a real config at the specifier path so only the path-guard fails open.
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":                  `{"extends": "@tsconfig/node18/tsconfig.json"}`,
		"@tsconfig/node18/tsconfig.json": `{"files": ["src/app.ts"]}`,
		"src/app.ts":                     "export const app = 1\n",
		"test/app.ts":                    "export const test = 1\n",
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
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when extends is a scoped npm package specifier", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q (a scoped npm-package extends target must fail open, same as no tsconfig.json)", file.Path, file.SourceScope, SourceScopeUnknown)
		}
	}
}

func TestApplySourceScopeTSConfigExtendsBareNpmSpecifierFailsOpen(t *testing.T) {
	// Plant a real config at the bare name so only the path-guard fails open.
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":    `{"extends": "some-base-config"}`,
		"some-base-config": `{"files": ["src/app.ts"]}`,
		"src/app.ts":       "export const app = 1\n",
		"test/app.ts":      "export const test = 1\n",
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
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when extends is a bare, unscoped npm-style package specifier", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q (a bare npm-package extends target must fail open, same as no tsconfig.json)", file.Path, file.SourceScope, SourceScopeUnknown)
		}
	}
}

func TestApplySourceScopeTSConfigExtendsChainHittingNpmSpecifierMidChainFailsOpen(t *testing.T) {
	// Mid-chain npm hop must fail the whole chain; plant a real file at the hop path.
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":                      `{"extends": "./sub/tsconfig.json"}`,
		"sub/tsconfig.json":                  `{"extends": "@tsconfig/node18/tsconfig.json"}`,
		"sub/@tsconfig/node18/tsconfig.json": `{"files": ["src/app.ts"]}`,
		"src/app.ts":                         "export const app = 1\n",
		"test/app.ts":                        "export const test = 1\n",
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
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when an npm-package specifier appears mid-chain", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q (an npm-package extends target mid-chain must fail the whole chain open)", file.Path, file.SourceScope, SourceScopeUnknown)
		}
	}
}

func TestApplySourceScopeTSConfigExtendsAbsolutePathOutsideSnapshotFailsOpen(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json": `{"extends": "/etc/passwd"}`,
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
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when extends is an absolute path outside the snapshot", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q (extends outside the snapshot must fail open, same as no tsconfig.json)", file.Path, file.SourceScope, SourceScopeUnknown)
		}
	}
}

func TestApplySourceScopeTSConfigExtendsSymlinkEscapingSnapshotFailsOpen(t *testing.T) {
	// Lexically in-bounds symlink to a host path must fail open (not read through).
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

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "src/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
		{Path: "test/app.ts", Status: "modified", Language: semantics.LanguageTypeScript},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ApplySourceScope() = %#v, want both files retained as unknown when the extends target is a symlink escaping the snapshot", files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeUnknown {
			t.Errorf("%s source scope = %q, want %q (a symlinked extends target escaping the snapshot must fail open rather than reading through it)", file.Path, file.SourceScope, SourceScopeUnknown)
		}
	}
}

func TestApplySourceScopeRebasesInheritedExcludeToBaseDirectory(t *testing.T) {
	// Rebased exclude hits packages/shared/test/, not root test/.
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

func TestApplySourceScopeIncludeAndFilesAreAdditive(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"tsconfig.json":         `{"files": ["src/explicit.ts"], "include": ["src/included/**/*.ts"]}`,
		"src/explicit.ts":       "export const explicit = 1\n",
		"src/included/extra.ts": "export const extra = 1\n",
		"src/other.ts":          "export const other = 1\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
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

func TestApplySourceScopeArrayExtendsPreservesChildOwnSettings(t *testing.T) {
	// Array extends is ignored (not multi-merged); child exclude must still apply.
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

func TestLoadTSConfigRebasesExtendsPatternsWhenDirIsASymlink(t *testing.T) {
	// snapshotRoot and baseDir must share resolved path space for rebase math.
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

func TestTSConfigExplicitEmptyFilesSelectsNoFiles(t *testing.T) {
	emptyFiles := []string{}
	config := tsConfig{Files: &emptyFiles}
	if config.matchesInclude("src/app.ts") {
		t.Fatal("matchesInclude() = true, want false for an explicit empty files setting")
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

func TestApplySourceScopeTalliesExcludedFiles(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"shipping/shipping.go":      "package shipping\n\nfunc Update() {}\n",
		"shipping/shipping_test.go": "package shipping\n\nfunc TestUpdate() {}\n",
	})
	head := scopeTestCommit(t, repo)

	kept, excluded, err := ApplySourceScope(repo, head, "", "production", []SelectedFile{
		{Path: "shipping/shipping.go", Status: "modified", Language: semantics.LanguageGo},
		{Path: "shipping/shipping_test.go", Status: "modified", Language: semantics.LanguageGo},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}

	if len(kept) != 1 || kept[0].Path != "shipping/shipping.go" || kept[0].SourceScope != SourceScopeUnknown {
		t.Fatalf("ApplySourceScope() kept = %#v, want only shipping.go", kept)
	}

	if len(excluded) != 1 || excluded[0].Reason != SourceScopeTestOnly || excluded[0].Language != string(semantics.LanguageGo) || excluded[0].Count != 1 {
		t.Fatalf("ApplySourceScope() excluded = %#v, want one test_only/go group of count 1", excluded)
	}
}

func TestApplySourceScopeResolvesGoTargetFromInvocationSubdirectory(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"go.mod":              "module example.com/scope-test\n\ngo 1.24\n",
		"cmd/app/main.go":     "package main\n\nimport \"example.com/scope-test/internal/app\"\n\nfunc main() { app.Run() }\n",
		"internal/app/app.go": "package app\n\nfunc Run() {}\n",
	})
	head := scopeTestCommit(t, repo)

	files, _, err := ApplySourceScope(filepath.Join(repo, "cmd", "app"), head, ".", "production", []SelectedFile{
		{Path: "cmd/app/main.go", Status: "modified", Language: semantics.LanguageGo},
		{Path: "internal/app/app.go", Status: "modified", Language: semantics.LanguageGo},
	})
	if err != nil {
		t.Fatalf("ApplySourceScope() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ApplySourceScope() kept %d files, want 2: %#v", len(files), files)
	}
	for _, file := range files {
		if file.SourceScope != SourceScopeProduction {
			t.Errorf("%s source scope = %q, want %q", file.Path, file.SourceScope, SourceScopeProduction)
		}
	}
}

func TestApplyBaselineSourceScopeTalliesExcludedFiles(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"shipping/shipping.go":      "package shipping\n\nfunc Update() {}\n",
		"shipping/shipping_test.go": "package shipping\n\nfunc TestUpdate() {}\n",
	})
	head := scopeTestCommit(t, repo)

	kept, excluded, err := ApplyBaselineSourceScope(repo, head, "", "production", []SelectedFile{
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

func TestApplyBaselineSourceScopeAllReturnsEverythingUnexcluded(t *testing.T) {
	repo := newScopeTestRepo(t, map[string]string{
		"shipping/shipping.go":      "package shipping\n\nfunc Update() {}\n",
		"shipping/shipping_test.go": "package shipping\n\nfunc TestUpdate() {}\n",
	})
	head := scopeTestCommit(t, repo)

	kept, excluded, err := ApplyBaselineSourceScope(repo, head, "", "all", []SelectedFile{
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

func newScopeTestRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	runScopeTestCommand(t, repo, "git", "init")
	for path, content := range files {
		writeScopeTestFile(t, repo, path, content)
	}
	return repo
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

func scopeTestCommit(t *testing.T, repo string) string {
	t.Helper()
	runScopeTestCommand(t, repo, "git", "add", ".")
	runScopeTestCommand(t, repo, "git", "-c", "user.name=scope-test", "-c", "user.email=scope-test@example.com", "commit", "-m", "fixture")
	return strings.TrimSpace(runScopeTestCommand(t, repo, "git", "rev-parse", "HEAD"))
}

func runScopeTestCommand(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
	return string(output)
}
