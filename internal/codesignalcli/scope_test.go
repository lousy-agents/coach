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
	// The base's exclude pattern ("test/**/*.ts") is resolved relative to the
	// base config's OWN directory (base/), so the file it must reach lives at
	// base/test/app.ts, not a root-level test/app.ts (real TypeScript never
	// rebases a root-level file into a subdirectory-declared base's scope).
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
	// The child specifies its own "include" (which must entirely replace, not
	// merge with, the base's "include") but omits "exclude" (which must
	// still be inherited from the base). The base's exclude pattern
	// ("src/excluded/**/*.ts") is resolved relative to the base's OWN
	// directory (base/), so the file it must reach lives at
	// base/src/excluded/fixture.ts, not a root-level src/excluded/fixture.ts.
	// The child's own include is widened to also reach under base/ so that
	// file is a candidate for inclusion at all, isolating the assertion to
	// whether the base's inherited-and-rebased exclude, specifically,
	// removes it.
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
	// tsconfig.json -> mid/tsconfig.json -> mid/root/tsconfig.json, where
	// only the outermost base (root) sets "exclude". Neither the package
	// config nor the middle config specifies include/exclude/files of its
	// own, so the root's exclude must still apply all the way through the
	// chain. root is nested under mid (rather than a sibling of it) so each
	// hop stays within the directory containing the config that references
	// it, per the existing extends security boundary. The root's exclude
	// pattern ("test/**/*.ts") is resolved relative to the root config's own
	// directory (mid/root/), so the file it must reach lives at
	// mid/root/test/app.ts, not a root-level test/app.ts.
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
	// A common monorepo pattern: the package's tsconfig.json extends a
	// config one directory DOWN (packages/foo), which in turn extends a
	// shared base config back UP at the snapshot root
	// ("../../tsconfig.base.json"). The final target is the snapshot root's
	// own tsconfig.base.json, which never actually leaves the snapshot, so
	// this must resolve successfully even though the second hop's own
	// directory (packages/foo) is not itself an ancestor of the target —
	// the escape check must be relative to the snapshot root, not to
	// whichever subdirectory happens to contain the current hop's config.
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
	// Two configs in the same directory extend each other directly, forming
	// a genuine cycle that stays entirely in-bounds (same directory as each
	// other and as the snapshot root) at every hop, so only the
	// cycle-detection guard — not the per-hop security boundary check — can
	// stop resolution here. Without cycle detection this would recurse
	// between tsconfig.json and base.json indefinitely.
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
	// A real, resolvable tsconfig.json is planted AT the npm-specifier-shaped
	// path itself (resolveExtendedTSConfig would join dir with the bare
	// extends value and read that path directly, with no "append
	// tsconfig.json" step for a bare/scoped specifier). If
	// isTSConfigPathSpecifier's guard were ever removed, this file would be
	// found and read, its "files" setting would apply, and the assertion
	// below would fail (src/app.ts would become production and test/app.ts
	// would be dropped) — proving the guard, not a missing file, is what
	// makes this fail open.
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
	// As above: a real, resolvable tsconfig-shaped file lives at the literal
	// path "some-base-config" (resolveExtendedTSConfig treats a bare extends
	// value as the config file itself, not a directory to search within), so
	// this test can only pass because isTSConfigPathSpecifier rejects the
	// bare specifier before ever trying to read it — not because the path
	// happens to not exist.
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
	// The root config extends a subdirectory config with no issue of its own;
	// that subdirectory config in turn extends an npm-package specifier. The
	// npm specifier must fail the whole chain open, not just the hop that
	// encountered it, even though the first hop (root -> sub) is a perfectly
	// resolvable path-shaped extends. As in the two tests above, a real
	// tsconfig-shaped file is planted at the mid-chain hop's
	// npm-specifier-shaped path (resolved relative to sub/, i.e.
	// "sub/@tsconfig/node18/tsconfig.json") so this test can only pass
	// because the guard rejects the specifier, not because nothing was
	// there to read.
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
	// The literal extends target ("./base.json") is lexically in-bounds, but
	// it is committed as a symlink pointing outside the snapshot entirely.
	// The escape check must resolve symlinks before judging containment, and
	// must read the resolved path rather than following the symlink
	// transparently via a raw os.ReadFile on the pre-resolution path.
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
	// The base config's exclude pattern is resolved relative to the base's
	// OWN directory (packages/shared/), so it must reach
	// packages/shared/test/fixture.ts but must NOT reach a root-level
	// test/root.ts that merely happens to share the pattern's relative
	// suffix ("test/**/*.ts").
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
	// Real TypeScript combines "files" and "include": the effective file set
	// is the union of explicit "files" entries and files matched by
	// "include" patterns, not an either/or choice between the two.
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
	// TypeScript 5.0+'s multi-base array-form "extends" is out of scope for
	// v1 (no merging of multiple bases), but an array-valued extends must not
	// regress to discarding the CHILD's own settings: before Extends existed
	// as a typed field, an array-valued extends was just an unrecognized
	// field encoding/json silently ignored, so the child's own include/
	// exclude/files still worked.
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
	// TypeScript resolves an extensionless relative extends target by trying
	// the literal path, then retrying with ".json" appended.
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
	// dir passed to loadTSConfig is the raw snapshot directory (ultimately
	// os.MkdirTemp's result), which on some platforms (notably macOS, where
	// /var is a symlink to /private/var) may itself contain a symlink
	// component. resolveExtendedTSConfig always returns a symlink-resolved
	// baseDir, so snapshotRoot must be resolved the same way before it is
	// used as the other half of the filepath.Rel(snapshotRoot, baseDir) math
	// in rebaseTSConfigPatterns -- otherwise the rebased pattern is computed
	// between an unresolved root and a resolved baseDir and comes out
	// nonsensical, silently defeating the inherited exclude pattern. This
	// reproduces that interaction directly, without needing a genuinely
	// symlinked OS temp directory: dir itself is a symlink pointing at a
	// real sibling directory.
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
