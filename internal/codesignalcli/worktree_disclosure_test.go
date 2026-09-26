package codesignalcli

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestWorktreeDisclosureDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		output []byte
		err    error
		want   []codesignal.Diagnostic
	}{
		{
			name: "clean tree adds no diagnostics",
		},
		{
			name:   "ignored files are not a dirty tree",
			output: porcelainZ("!! secret.go"),
		},
		{
			name:   "untracked go file is disclosed as untracked and not analyzed",
			output: porcelainZ("?? pending.go"),
			want: []codesignal.Diagnostic{{
				Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
				Message: "1 untracked file was not analyzed: pending.go",
			}},
		},
		{
			name:   "staged go file is disclosed as staged and not analyzed",
			output: porcelainZ("A  indexed.go"),
			want: []codesignal.Diagnostic{{
				Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
				Message: "1 staged file was not analyzed: indexed.go",
			}},
		},
		{
			name:   "modified go file is disclosed as modified and not analyzed",
			output: porcelainZ(" M a.go"),
			want: []codesignal.Diagnostic{{
				Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
				Message: "1 modified file was not analyzed: a.go",
			}},
		},
		{
			name:   "typescript and tsx extensions are supported languages",
			output: porcelainZ("?? widget.tsx", "?? helper.ts"),
			want: []codesignal.Diagnostic{{
				Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
				Message: "2 untracked files were not analyzed: helper.ts, widget.tsx",
			}},
		},
		{
			name:   "extension case does not hide a supported file",
			output: porcelainZ("?? Pending.GO"),
			want: []codesignal.Diagnostic{{
				Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
				Message: "1 untracked file was not analyzed: Pending.GO",
			}},
		},
		{
			name:   "a path staged and modified is disclosed in both categories",
			output: porcelainZ("MM both.go"),
			want: []codesignal.Diagnostic{
				{
					Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
					Message: "1 staged file was not analyzed: both.go",
				},
				{
					Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
					Message: "1 modified file was not analyzed: both.go",
				},
			},
		},
		{
			name:   "rename discloses the new path only",
			output: porcelainZ("R  dst.go", "src.go"),
			want: []codesignal.Diagnostic{{
				Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
				Message: "1 staged file was not analyzed: dst.go",
			}},
		},
		{
			name:   "unmerged UU go file is disclosed as unmerged and not analyzed",
			output: porcelainZ("UU conflict.go"),
			want: []codesignal.Diagnostic{{
				Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
				Message: "1 unmerged file was not analyzed: conflict.go",
			}},
		},
		{
			name:   "unmerged UA is unmerged only, not staged",
			output: porcelainZ("UA added.go"),
			want: []codesignal.Diagnostic{{
				Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
				Message: "1 unmerged file was not analyzed: added.go",
			}},
		},
		{
			name:   "unsupported files are a dirty tree without a not-analyzed kind",
			output: porcelainZ("?? readme.txt", "?? notes.md"),
			want: []codesignal.Diagnostic{{
				Kind:    codesignal.DiagKindWorktreeNotClean,
				Message: "working tree is not clean: notes.md, readme.txt",
			}},
		},
		{
			name:   "supported files suppress the unsupported-only notice",
			output: porcelainZ("?? notes.md", "?? pending.go"),
			want: []codesignal.Diagnostic{{
				Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
				Message: "1 untracked file was not analyzed: pending.go",
			}},
		},
		{
			name: "status check failure is one diagnostic and does not fail closed into not-analyzed",
			err:  errors.New("git status failed"),
			want: []codesignal.Diagnostic{{
				Kind:    codesignal.DiagKindWorktreeStatusCheckFailed,
				Message: "working tree status check failed: git status failed",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withDirtyWorktreeGit(t, func(string, ...string) ([]byte, error) {
				return tt.output, tt.err
			})

			got := WorkingTreeDisclosureDiagnostics("repo")
			if len(got) != len(tt.want) {
				t.Fatalf("diagnostics: got %d (%+v), want %d (%+v)", len(got), got, len(tt.want), tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("diagnostic %d: got %+v, want %+v", i, got[i], tt.want[i])
				}
				if got[i].Path != "" {
					t.Errorf("diagnostic %d path: got %q, want empty so files_unanalyzed stays zero", i, got[i].Path)
				}
			}
		})
	}
}

func TestWorktreeDisclosureDiagnosticsBoundsTheSample(t *testing.T) {
	var records []string
	for i := 0; i < 20; i++ {
		records = append(records, "?? bulk"+twoDigits(i)+".go")
	}
	withDirtyWorktreeGit(t, func(string, ...string) ([]byte, error) {
		return porcelainZ(records...), nil
	})

	got := WorkingTreeDisclosureDiagnostics("repo")
	if len(got) != 1 {
		t.Fatalf("diagnostics: got %d (%+v), want 1 grouped diagnostic", len(got), got)
	}
	if got[0].Kind != codesignal.DiagKindWorktreeChangesNotAnalyzed {
		t.Fatalf("kind: got %q, want %q", got[0].Kind, codesignal.DiagKindWorktreeChangesNotAnalyzed)
	}
	message := got[0].Message
	if !strings.HasPrefix(message, "20 untracked files were not analyzed: ") {
		t.Fatalf("message: got %q, want a total count of 20 and an untracked classification", message)
	}
	if strings.Contains(message, "bulk05.go") {
		t.Fatalf("message listed a path past the sample: %q", message)
	}
	for _, name := range []string{"bulk00.go", "bulk04.go"} {
		if !strings.Contains(message, name) {
			t.Errorf("message missing sample path %s: %q", name, message)
		}
	}
	if strings.Contains(message, "staged") || strings.Contains(message, "modified") {
		t.Fatalf("untracked-only message named another classification: %q", message)
	}
}

func withDirtyWorktreeGit(t *testing.T, fn func(dir string, args ...string) ([]byte, error)) {
	t.Helper()
	original := runDirtyWorktreeGit
	runDirtyWorktreeGit = fn
	t.Cleanup(func() { runDirtyWorktreeGit = original })
}

func porcelainZ(records ...string) []byte {
	var b strings.Builder
	for _, record := range records {
		b.WriteString(record)
		b.WriteByte(0)
	}
	return []byte(b.String())
}

func twoDigits(n int) string {
	return string([]byte{byte('0' + n/10), byte('0' + n%10)})
}
