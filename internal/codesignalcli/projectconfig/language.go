package projectconfig

import (
	"bytes"
	"encoding/json"
)

// Language is the analysis a policy's roots belong to. The zero value is a policy that does not say.
type Language string

const (
	LanguageGo         Language = "go"
	LanguageTypeScript Language = "typescript"
)

const languageRefusal = `the "language" key must be "go" or "typescript"`

// InvalidLanguageError is the refusal of a language outside the supported set. Its words quote nothing from the
// policy, so a caller may print them as they stand.
type InvalidLanguageError struct{}

func (*InvalidLanguageError) Error() string { return languageRefusal }

// UnmarshalJSON is strict where the key changes which analysis runs: null and an empty string are refused, not read as
// absent.
func (l *Language) UnmarshalJSON(data []byte) error {
	var value string
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) || json.Unmarshal(data, &value) != nil {
		return &InvalidLanguageError{}
	}
	switch Language(value) {
	case LanguageGo, LanguageTypeScript:
		*l = Language(value)
		return nil
	}
	return &InvalidLanguageError{}
}
