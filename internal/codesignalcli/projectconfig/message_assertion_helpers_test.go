package projectconfig

import (
	"strings"
	"testing"
)

func requireContains(t *testing.T, message string, words ...string) {
	t.Helper()
	for _, want := range words {
		if !strings.Contains(message, want) {
			t.Errorf("message %q; want it to contain %q", message, want)
		}
	}
}
