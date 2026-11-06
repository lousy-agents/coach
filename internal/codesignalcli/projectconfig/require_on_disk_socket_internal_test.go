//go:build unix

package projectconfig

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A Unix-domain socket is neither a regular file nor something to open: it is refused as an entry of
// another kind, and the check does not connect to it. The socket lives under /tmp because the system
// limits the length of a socket path, which a test's own temporary directory can exceed.
func TestCheckRegularFileUnderRefusesAUnixSocketAtThePolicyPath(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "pc")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	listener, err := net.Listen("unix", filepath.Join(dir, "p.json"))
	if err != nil {
		t.Skipf("cannot create a unix socket here: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	got := checkRegularFileUnder(dir, "p.json")

	var configErr *ConfigError
	if !errors.As(got, &configErr) || configErr.Kind != KindInvalid || !strings.Contains(configErr.Message, "is not a regular file") {
		t.Errorf("checkRegularFileUnder = %v; want a project_config_invalid error saying the entry is not a regular file", got)
	}
}
