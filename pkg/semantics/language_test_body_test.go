package semantics

import "testing"

func body_languageTest_24(t *testing.T, tt struct {
	ext      string
	wantLang Language
	wantOK   bool
}) {
	gotLang, gotOK := LanguageForExtension(tt.ext)
	if gotLang != tt.wantLang || gotOK != tt.wantOK {
		t.Errorf("LanguageForExtension(%q): got (%q, %v), want (%q, %v)", tt.ext, gotLang, gotOK, tt.wantLang, tt.wantOK)
	}
}
