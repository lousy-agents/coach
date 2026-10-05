package semantics

import "testing"

type languageForExtensionCase struct {
	ext      string
	wantLang Language
	wantOK   bool
}

func expectLanguageForExtension(t *testing.T, tt languageForExtensionCase) {
	gotLang, gotOK := LanguageForExtension(tt.ext)
	if gotLang != tt.wantLang || gotOK != tt.wantOK {
		t.Errorf("LanguageForExtension(%q): got (%q, %v), want (%q, %v)", tt.ext, gotLang, gotOK, tt.wantLang, tt.wantOK)
	}
}
