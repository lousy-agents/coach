package acceptancestyle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func body_checkTest_127(t *testing.T, tt struct {
	name           string
	files          map[string]string
	wantViolations int
	wantPathSubstr string
}) {
	t.Parallel()
	root := t.TempDir()
	for rel, body := range tt.files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := Check(root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(got) != tt.wantViolations {
		t.Fatalf("violations = %v, want %d", got, tt.wantViolations)
	}
	if tt.wantViolations > 0 {
		if tt.wantPathSubstr != "" && !strings.Contains(got[0].Path, tt.wantPathSubstr) {
			t.Errorf("path %q does not contain %q", got[0].Path, tt.wantPathSubstr)
		}
		if got[0].Reason == "" {
			t.Error("expected non-empty reason")
		}
	}
}
