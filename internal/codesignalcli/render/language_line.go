package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// renderLanguageLine says which language the project analysis ran in when the flag or the policy chose it. A scan that
// read no project policy ran none, and a policy that names no language leaves the text as it was before the key
// existed: the JSON report carries the default.
func renderLanguageLine(b *strings.Builder, language *codesignal.ProjectLanguageScope) {
	if language == nil || language.Source == codesignal.ProjectLanguageSourceDefault {
		return
	}
	fmt.Fprintf(b, "language: %s (%s)\n", terminalText(language.Language), languageSourceWords(language.Source))
}

func languageSourceWords(source codesignal.ProjectLanguageSource) string {
	switch source {
	case codesignal.ProjectLanguageSourceFlag:
		return "from --project-language"
	}
	return "from the policy"
}
