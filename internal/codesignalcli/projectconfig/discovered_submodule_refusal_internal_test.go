package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// Only a path inside a submodule is the submodule's refusal: an unusable default policy of any other kind is the disk's
// to word, and a selection with none refuses nothing.
func TestOnlyADefaultPolicyInsideASubmoduleIsRefusedAsTheSubmodules(t *testing.T) {
	inside := &ConfigError{PolicyCause: projectreadiness.CauseInSubmodule, Blocker: "vendor/sub"}
	notAFile := &ConfigError{PolicyCause: projectreadiness.CauseNotARegularFile}

	if err := (Selection{}).SubmoduleRefusal(); err != nil {
		t.Errorf("SubmoduleRefusal with no unusable default policy = %v; want nil", err)
	}
	if err := (Selection{UnusableCanonical: notAFile}).SubmoduleRefusal(); err != nil {
		t.Errorf("SubmoduleRefusal for an unusable default policy outside a submodule = %v; want nil", err)
	}
	err := (Selection{UnusableCanonical: inside}).SubmoduleRefusal()
	var refusal *ConfigError
	if !errors.As(err, &refusal) || refusal.PolicyCause != projectreadiness.CauseInSubmodule || !strings.Contains(refusal.Message, `the submodule "vendor/sub"`) {
		t.Errorf("SubmoduleRefusal inside a submodule = %v; want the submodule named with the cause in_submodule", err)
	}
}
