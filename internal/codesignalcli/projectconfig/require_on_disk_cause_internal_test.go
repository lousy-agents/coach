package projectconfig

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestCauseTextNeverRepeatsThePathInTheError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"a permission error carrying a path with an escape character", &fs.PathError{Op: "statat", Path: "ev\x1b[31mRED/p.json", Err: syscall.EACCES}, "permission denied"},
		{"an error that is not an errno", errors.New("statat ev\x1b[31mRED/p.json: something else"), "unexpected file-system error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertCauseText(t, c.err, c.want)
		})
	}
}

func assertCauseText(t *testing.T, err error, want string) {
	t.Helper()
	if got := causeText(err); got != want {
		t.Errorf("causeText(%v) = %q, want %q", err, got, want)
	}
}

func TestCheckRegularFileUnderRefusesAFileThatCannotBeRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a mode 000 file, so this state cannot be built")
	}
	root := t.TempDir()
	full := filepath.Join(root, "pol.json")
	if err := os.WriteFile(full, []byte("{}"), 0o000); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(full, 0o644) })

	err := checkRegularFileUnder(root, "pol.json")

	var configErr *ConfigError
	if !errors.As(err, &configErr) || configErr.Kind != KindInvalid || configErr.Remedy != projectreadiness.RemedyNone {
		t.Fatalf("checkRegularFileUnder = %v; want an invalid-policy error that offers no authoring", err)
	}
	if !strings.Contains(configErr.Message, "cannot be read on disk: permission denied") {
		t.Errorf("message %q does not name the cause", configErr.Message)
	}
}
