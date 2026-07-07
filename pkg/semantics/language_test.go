package semantics

import "testing"

// AC-R2.5: LanguageForExtension must map known extensions case-insensitively
// and report ("", false) for anything unrecognized. AnalyzeBytes never
// calls this helper itself (it is additive, caller-only).
func TestLanguageForExtension(t *testing.T) {
	tests := []struct {
		ext      string
		wantLang Language
		wantOK   bool
	}{
		{ext: ".ts", wantLang: LanguageTypeScript, wantOK: true},
		{ext: ".TS", wantLang: LanguageTypeScript, wantOK: true},
		{ext: ".tsx", wantLang: LanguageTSX, wantOK: true},
		{ext: ".go", wantLang: LanguageGo, wantOK: true},
		{ext: ".js", wantLang: "", wantOK: false},
		{ext: ".py", wantLang: "", wantOK: false},
		{ext: "", wantLang: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			gotLang, gotOK := LanguageForExtension(tt.ext)
			if gotLang != tt.wantLang || gotOK != tt.wantOK {
				t.Errorf("LanguageForExtension(%q): got (%q, %v), want (%q, %v)", tt.ext, gotLang, gotOK, tt.wantLang, tt.wantOK)
			}
		})
	}
}
