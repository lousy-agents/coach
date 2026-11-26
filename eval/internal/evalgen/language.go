package evalgen

import "fmt"

// Language is the language of a generated program.
type Language string

const (
	LanguageGo         Language = "go"
	LanguageTypeScript Language = "typescript"
)

// Languages lists the languages a program can be generated in.
var Languages = []Language{LanguageGo, LanguageTypeScript}

// ParseLanguage names the values it accepts when the input is none of them.
func ParseLanguage(text string) (Language, error) {
	for _, language := range Languages {
		if string(language) == text {
			return language, nil
		}
	}
	return "", fmt.Errorf("language must be %s or %s, got %q", LanguageGo, LanguageTypeScript, text)
}

// Version is the generator version of a language; it changes whenever that language's trees can change.
func (l Language) Version() string {
	if l == LanguageTypeScript {
		return TypeScriptGeneratorVersion
	}
	return GeneratorVersion
}
