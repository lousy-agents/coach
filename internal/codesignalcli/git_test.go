package codesignalcli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestParseNameStatusZ(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    []nameStatusRecord
		wantErr bool
	}{
		{
			name:    "empty",
			payload: []byte{},
			want:    nil,
		},
		{
			name:    "added",
			payload: joinNUL("A", "a.go"),
			want:    []nameStatusRecord{{status: "A", paths: []string{"a.go"}}},
		},
		{
			name:    "modified",
			payload: joinNUL("M", "a.go"),
			want:    []nameStatusRecord{{status: "M", paths: []string{"a.go"}}},
		},
		{
			name:    "deleted",
			payload: joinNUL("D", "a.go"),
			want:    []nameStatusRecord{{status: "D", paths: []string{"a.go"}}},
		},
		{
			name:    "rename with score consumes two paths",
			payload: joinNUL("R100", "old.go", "new.go"),
			want:    []nameStatusRecord{{status: "R100", paths: []string{"old.go", "new.go"}}},
		},
		{
			name:    "copy with score consumes two paths",
			payload: joinNUL("C75", "src.go", "dst.go"),
			want:    []nameStatusRecord{{status: "C75", paths: []string{"src.go", "dst.go"}}},
		},
		{
			name:    "type change other status consumes one path",
			payload: joinNUL("T", "link.go"),
			want:    []nameStatusRecord{{status: "T", paths: []string{"link.go"}}},
		},
		{
			name: "record alignment not thrown off by preceding multi-path record",
			payload: joinNUL(
				"R100", "old.go", "new.go",
				"M", "unrelated.go",
			),
			want: []nameStatusRecord{
				{status: "R100", paths: []string{"old.go", "new.go"}},
				{status: "M", paths: []string{"unrelated.go"}},
			},
		},
		{
			name:    "empty status is malformed",
			payload: joinNUL("", "a.go"),
			wantErr: true,
		},
		{
			name:    "truncated rename record is malformed",
			payload: joinNUL("R100", "old.go"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNameStatusZ(tt.payload)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseNameStatusZ(%q): want error, got nil", tt.payload)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseNameStatusZ(%q): unexpected error: %v", tt.payload, err)
			}
			if !recordsEqual(got, tt.want) {
				t.Errorf("parseNameStatusZ(%q) = %#v, want %#v", tt.payload, got, tt.want)
			}
		})
	}
}

func TestParseNameStatusZUnusualPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "spaces", path: "a b/c d.go"},
		{name: "quotes", path: `a"b.go`},
		{name: "newline", path: "a\nb.go"},
		{name: "non-ascii", path: "café/日本語.go"},
		{name: "shell metacharacters", path: "$(rm -rf /); `echo pwned`; a&&b.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := joinNUL("M", tt.path)
			got, err := parseNameStatusZ(payload)
			if err != nil {
				t.Fatalf("parseNameStatusZ: unexpected error: %v", err)
			}
			if len(got) != 1 || got[0].paths[0] != tt.path {
				t.Fatalf("parseNameStatusZ(%q) = %#v, want single record with path %q", payload, got, tt.path)
			}
		})
	}
}

func TestSelectChangedFilesLanguageFiltering(t *testing.T) {
	dir := newTempGitRepoT(t)
	initialSHA := commitFileT(t, dir, "keep.go", "package keep\n")
	commitFileT(t, dir, "keep.go", "package keep\n\nfunc F() {}\n")
	commitFileT(t, dir, "unsupported.txt", "plain text\n")

	selected, diagnostics, err := SelectChangedFiles(dir, initialSHA)
	if err != nil {
		t.Fatalf("SelectChangedFiles: unexpected error: %v", err)
	}

	if len(selected) != 1 {
		t.Fatalf("selected = %#v, want exactly one supported-language file", selected)
	}
	if selected[0].Path != "keep.go" || selected[0].Language != semantics.LanguageGo || selected[0].Status != "modified" {
		t.Errorf("selected[0] = %#v, want keep.go/modified/go", selected[0])
	}

	if len(diagnostics) != 1 || diagnostics[0].Kind != "unsupported_language" || diagnostics[0].Path != "unsupported.txt" {
		t.Errorf("diagnostics = %#v, want one unsupported_language diagnostic for unsupported.txt", diagnostics)
	}
}

func TestSelectChangedFilesRenameEmitsUnsupportedChangeType(t *testing.T) {
	dir := newTempGitRepoT(t)
	initialSHA := commitFileT(t, dir, "old.go", "package old\n// padding so rename detection kicks in\n// more padding\n// more padding\n// more padding\n")
	renameFileT(t, dir, "old.go", "new.go")

	selected, diagnostics, err := SelectChangedFiles(dir, initialSHA)
	if err != nil {
		t.Fatalf("SelectChangedFiles: unexpected error: %v", err)
	}

	if len(selected) != 0 {
		t.Errorf("selected = %#v, want none for a pure rename", selected)
	}

	found := false
	for _, d := range diagnostics {
		if d.Kind == "unsupported_change_type" && d.Path == "new.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("diagnostics = %#v, want one unsupported_change_type diagnostic for new.go", diagnostics)
	}
}

func TestResolveRevisions(t *testing.T) {
	t.Run("valid base", func(t *testing.T) {
		dir := newTempGitRepoT(t)
		initialSHA := commitFileT(t, dir, "a.go", "package a\n")
		headSHA := commitFileT(t, dir, "b.go", "package a\n\nfunc B() {}\n")

		gotHead, gotMergeBase, err := ResolveRevisions(dir, initialSHA)
		if err != nil {
			t.Fatalf("ResolveRevisions: unexpected error: %v", err)
		}
		if gotHead != headSHA {
			t.Errorf("headSHA = %q, want %q", gotHead, headSHA)
		}
		if gotMergeBase != initialSHA {
			t.Errorf("mergeBaseSHA = %q, want %q", gotMergeBase, initialSHA)
		}
	})

	t.Run("invalid base", func(t *testing.T) {
		dir := newTempGitRepoT(t)
		commitFileT(t, dir, "a.go", "package a\n")

		_, _, err := ResolveRevisions(dir, "doesnotexist12345")
		if err == nil {
			t.Fatal("ResolveRevisions: want error for unresolvable base, got nil")
		}
		var opErr *OperationalError
		if !isOperationalError(err, &opErr) {
			t.Errorf("ResolveRevisions error = %v, want *OperationalError", err)
		}
	})

	t.Run("non-worktree directory", func(t *testing.T) {
		dir := t.TempDir()

		_, _, err := ResolveRevisions(dir, "HEAD")
		if err == nil {
			t.Fatal("ResolveRevisions: want error for non-worktree directory, got nil")
		}
		var opErr *OperationalError
		if !isOperationalError(err, &opErr) {
			t.Errorf("ResolveRevisions error = %v, want *OperationalError", err)
		}
	})
}

func joinNUL(fields ...string) []byte {
	return []byte(strings.Join(fields, "\x00") + "\x00")
}

func recordsEqual(a, b []nameStatusRecord) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].status != b[i].status {
			return false
		}
		if len(a[i].paths) != len(b[i].paths) {
			return false
		}
		for j := range a[i].paths {
			if a[i].paths[j] != b[i].paths[j] {
				return false
			}
		}
	}
	return true
}

func isOperationalError(err error, target **OperationalError) bool {
	if opErr, ok := err.(*OperationalError); ok {
		*target = opErr
		return true
	}
	return false
}

func newTempGitRepoT(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}

	return dir
}

var commitTestEnv = append(os.Environ(),
	"GIT_AUTHOR_NAME=coach-test",
	"GIT_AUTHOR_EMAIL=coach-test@example.com",
	"GIT_COMMITTER_NAME=coach-test",
	"GIT_COMMITTER_EMAIL=coach-test@example.com",
)

func commitFileT(t *testing.T, dir, name, contents string) string {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}

	addCmd := exec.Command("git", "add", name)
	addCmd.Dir = dir
	if output, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add %s: %v: %s", name, err, output)
	}

	commitCmd := exec.Command("git", "commit", "-m", "commit "+name)
	commitCmd.Dir = dir
	commitCmd.Env = commitTestEnv
	if output, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit %s: %v: %s", name, err, output)
	}

	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = dir
	output, err := revCmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}

	return strings.TrimSpace(string(output))
}

func renameFileT(t *testing.T, dir, from, to string) {
	t.Helper()

	mvCmd := exec.Command("git", "mv", from, to)
	mvCmd.Dir = dir
	if output, err := mvCmd.CombinedOutput(); err != nil {
		t.Fatalf("git mv %s %s: %v: %s", from, to, err, output)
	}

	commitCmd := exec.Command("git", "commit", "-m", "rename "+from+" to "+to)
	commitCmd.Dir = dir
	commitCmd.Env = commitTestEnv
	if output, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit rename: %v: %s", err, output)
	}
}
