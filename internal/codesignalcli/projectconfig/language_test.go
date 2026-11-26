package projectconfig

import (
	"errors"
	"fmt"
	"testing"
)

func policyWithLanguage(raw string) []byte {
	return fmt.Appendf(nil, `{"schema_version":"1","roots":["."],"language":%s}`, raw)
}

func TestParseReadsTheLanguageTheKeyNames(t *testing.T) {
	for raw, want := range map[string]Language{`"go"`: LanguageGo, `"typescript"`: LanguageTypeScript} {
		config, err := Parse(policyWithLanguage(raw))
		if err != nil || config.Language != want {
			t.Errorf("language %s: got %q, %v; want %q", raw, config.Language, err, want)
		}
	}
}

func TestParseLeavesLanguageEmptyWhenThePolicyDoesNotSayAndNamesTheKeyWhenItIsWrong(t *testing.T) {
	config, err := Parse([]byte(`{"schema_version":"1","roots":["."]}`))
	if err != nil || config.Language != "" {
		t.Fatalf("a policy without the key: got %q, %v", config.Language, err)
	}
	for _, raw := range []string{`"Go"`, `"python"`, `""`, `5`, `true`, `["go"]`, `{}`, `null`} {
		_, err := Parse(policyWithLanguage(raw))
		var refusal *InvalidLanguageError
		if !errors.As(err, &refusal) {
			t.Errorf("language %s: got %v, want an InvalidLanguageError", raw, err)
		}
	}
}
