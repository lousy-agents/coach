package projectconfig

import (
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A repository root that cannot be opened is a path the process cannot read, said so and not walked as if it
// were there: the walk would otherwise start from a root that was never opened.
func TestARootThatCannotBeOpenedIsRefusedAsUnreadableOnDisk(t *testing.T) {
	err := checkRegularFileUnder(filepath.Join(t.TempDir(), "absent"), "cfg/p.json")

	refusal, ok := err.(*ConfigError)
	if !ok || refusal.PolicyCause != projectreadiness.CauseUnreadableOnDisk || refusal.Remedy != projectreadiness.RemedyNone {
		t.Errorf("checkRegularFileUnder = %v (%T); want a refusal with cause unreadable_on_disk and no remedy", err, err)
	}
}
