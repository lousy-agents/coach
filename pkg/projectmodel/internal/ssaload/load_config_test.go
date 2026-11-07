package ssaload

import (
	"context"
	"slices"
	"testing"
)

// The packages are loaded from a directory of committed bytes, by a go that would run git there to stamp
// version control information and would fetch and run a toolchain a committed go.mod names.
func TestTheModuleIsLoadedWithoutVersionControlStampingOrAToolchainFetch(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "auto")

	config := moduleLoadConfig(context.Background(), "/snapshot/module")

	if !slices.Contains(config.BuildFlags, "-buildvcs=false") {
		t.Errorf("BuildFlags = %q; want -buildvcs=false", config.BuildFlags)
	}
	if len(config.Env) == 0 || config.Env[len(config.Env)-1] != "GOTOOLCHAIN=local" {
		t.Errorf("Env ends with %q; want GOTOOLCHAIN=local last, so it wins over the caller's setting", config.Env[max(0, len(config.Env)-1):])
	}
	if config.Dir != "/snapshot/module" {
		t.Errorf("Dir = %q; want the module's directory", config.Dir)
	}
}
