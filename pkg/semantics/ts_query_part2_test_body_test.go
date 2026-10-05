package semantics

import (
	"testing"
)

func body_tsQueryPart2Test_23(t *testing.T, tt struct {
	name   string
	source string
}) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	imports, err := extractTSImports(languageRegistry[LanguageTypeScript].engineLang, root, []byte(tt.source))
	if err != nil {
		t.Fatalf("extractTSImports for %s %q: got err %v, want nil", tt.name, tt.source, err)
	}
	if len(imports) != 0 {
		t.Errorf("extractTSImports for %s %q: got %d imports (%+v), want 0 (out of scope for v1)", tt.name, tt.source, len(imports), imports)
	}
}
