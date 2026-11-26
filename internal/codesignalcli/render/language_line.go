package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// renderLanguageLine says which language the project analysis ran in and what chose it. A scan that read no project
// policy ran none, so it says nothing.
func renderLanguageLine(b *strings.Builder, language *codesignal.ProjectLanguageScope) {
	if language == nil {
		return
	}
	fmt.Fprintf(b, "language: %s (%s)\n", terminalText(language.Language), languageSourceWords(language.Source))
}

func languageSourceWords(source codesignal.ProjectLanguageSource) string {
	switch source {
	case codesignal.ProjectLanguageSourceFlag:
		return "from --project-language"
	case codesignal.ProjectLanguageSourcePolicy:
		return "from the policy"
	}
	return "the default"
}
