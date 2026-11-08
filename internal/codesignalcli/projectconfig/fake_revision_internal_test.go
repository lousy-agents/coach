package projectconfig

import (
	"errors"
	"testing"
)

// fakeRevision is a revision's tree as the names directly under each directory, "" being the root.
type fakeRevision map[string][]string

func (f fakeRevision) entries(directory string) ([]string, error) {
	return f[directory], nil
}

func configErrorOf(t *testing.T, err error) *ConfigError {
	t.Helper()
	var found *ConfigError
	if !errors.As(err, &found) {
		t.Fatalf("error = %v, want a *ConfigError", err)
	}
	return found
}
