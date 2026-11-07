package projectconfig

import (
	"strings"
	"testing"
)

// requireWords fails the test for each word that message does not contain.
func requireWords(t *testing.T, message string, words ...string) {
	t.Helper()
	for _, word := range words {
		if !strings.Contains(message, word) {
			t.Errorf("message %q; want it to contain %q", message, word)
		}
	}
}

// requireNoWords fails the test for each word that message contains.
func requireNoWords(t *testing.T, message string, words ...string) {
	t.Helper()
	for _, word := range words {
		if strings.Contains(message, word) {
			t.Errorf("message %q; want it not to contain %q", message, word)
		}
	}
}
