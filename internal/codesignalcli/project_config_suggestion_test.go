package codesignalcli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// TestSuggestPrimaryRootDiagnostic covers the diagnostic-code mapping and
// priority ordering from issue #220 that are impractical to trigger via a
// real Git repository at the acceptance-test level (root-discovery
// budget exhaustion, an unavailable snapshot, and multiple simultaneous
// diagnostic severities), per project_config_suggestion_acceptance_test.go
// covering the remaining rows end-to-end through the coach binary.
func TestSuggestPrimaryRootDiagnostic(t *testing.T) {
	tests := []struct {
		name     string
		result   projectmodel.RootDiscoveryResult
		wantCode string
		wantOK   bool
		wantPath string
	}{
		{
			name: "unavailable takes priority over everything else",
			result: projectmodel.RootDiscoveryResult{
				Complete: false,
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{
						{Code: projectmodel.DiagRootAmbiguous, Path: "nested"},
						{Code: projectmodel.DiagRootUnavailable, Path: "."},
					},
				},
			},
			wantCode: SuggestDiagSnapshotUnavailable,
			wantPath: ".",
		},
		{
			name: "outside_snapshot maps to ambiguous_roots",
			result: projectmodel.RootDiscoveryResult{
				Complete: true,
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{{Code: projectmodel.DiagRootOutsideSnapshot, Path: "../outside"}},
				},
			},
			wantCode: SuggestDiagAmbiguousRoots,
			wantPath: "../outside",
		},
		{
			name: "invalid maps to ambiguous_roots",
			result: projectmodel.RootDiscoveryResult{
				Complete: true,
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{{Code: projectmodel.DiagRootInvalid, Path: "bad/go.mod", Message: "parse error"}},
				},
			},
			wantCode: SuggestDiagAmbiguousRoots,
			wantPath: "bad/go.mod",
		},
		{
			name: "duplicate maps to ambiguous_roots",
			result: projectmodel.RootDiscoveryResult{
				Complete: true,
				Roots:    []string{"dup"},
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{{Code: projectmodel.DiagRootDuplicate, Path: "dup"}},
				},
			},
			wantCode: SuggestDiagAmbiguousRoots,
			wantPath: "dup",
		},
		{
			name: "ambiguous maps to ambiguous_roots",
			result: projectmodel.RootDiscoveryResult{
				Complete: true,
				Roots:    []string{"nested", "nested/inner"},
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{{Code: projectmodel.DiagRootAmbiguous, Path: "nested"}},
				},
			},
			wantCode: SuggestDiagAmbiguousRoots,
			wantPath: "nested",
		},
		{
			name: "ambiguous-family diagnostics outrank incomplete",
			result: projectmodel.RootDiscoveryResult{
				Complete: false,
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{
						{Code: projectmodel.DiagRootIncomplete},
						{Code: projectmodel.DiagRootDuplicate, Path: "dup"},
					},
				},
			},
			wantCode: SuggestDiagAmbiguousRoots,
			wantPath: "dup",
		},
		{
			name: "incomplete with an explicit DiagRootIncomplete diagnostic",
			result: projectmodel.RootDiscoveryResult{
				Complete: false,
				Coverage: projectmodel.Coverage{
					Diagnostics: []projectmodel.Diagnostic{{Code: projectmodel.DiagRootIncomplete}},
				},
			},
			wantCode: SuggestDiagIncomplete,
		},
		{
			name: "incomplete with no specific DiagRootIncomplete diagnostic still maps to incomplete",
			result: projectmodel.RootDiscoveryResult{
				Complete: false,
			},
			wantCode: SuggestDiagIncomplete,
		},
		{
			name: "a complete but empty root set maps to no_go_modules",
			result: projectmodel.RootDiscoveryResult{
				Complete: true,
				Roots:    nil,
			},
			wantCode: SuggestDiagNoGoModules,
		},
		{
			name: "a complete, non-empty root set is ok",
			result: projectmodel.RootDiscoveryResult{
				Complete: true,
				Roots:    []string{".", "services/payments"},
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, path, message, ok := suggestPrimaryRootDiagnostic(tt.result)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (code=%q message=%q)", ok, tt.wantOK, code, message)
			}
			if tt.wantOK {
				return
			}
			if code != tt.wantCode {
				t.Errorf("code = %q, want %q", code, tt.wantCode)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			if message == "" {
				t.Errorf("message must be non-empty and deterministic, got empty string")
			}
		})
	}
}

// TestSnapshotUnavailableMessageNeverLeaksAbsolutePath pins
// snapshotUnavailableMessage's contract directly at the unit level: no
// absolute host filesystem path may survive into the returned message,
// regardless of whether the underlying error is a plain string that
// happens to embed a known absolute path (as resolveHEAD's
// "not inside a Git worktree" and NewGoSnapshotFS's "git ls-tree failed
// ... in %q" both are) or an *fs.PathError carrying an absolute path the
// caller never supplied (as filepath.EvalSymlinks' failure inside
// repositoryRoot is).
func TestSnapshotUnavailableMessageNeverLeaksAbsolutePath(t *testing.T) {
	const absoluteDir = "/tmp/coach-acceptance-repo-123"

	t.Run("plain error embedding a known absolute path is stripped", func(t *testing.T) {
		err := fmt.Errorf("coach codesignal: %s is not inside a Git worktree", absoluteDir)
		got := snapshotUnavailableMessage("resolve HEAD", err, absoluteDir)
		if strings.Contains(got, absoluteDir) {
			t.Fatalf("snapshotUnavailableMessage(%q) = %q, still contains the absolute path", err, got)
		}
		if !strings.Contains(got, "resolve HEAD") {
			t.Errorf("snapshotUnavailableMessage(%q) = %q, want it to name the failing operation", err, got)
		}
	})

	t.Run("fs.PathError is unwrapped to its errno, discarding Path entirely", func(t *testing.T) {
		pathErr := &fs.PathError{Op: "lstat", Path: absoluteDir, Err: fs.ErrNotExist}
		wrapped := fmt.Errorf("resolving repository root: %w", pathErr)
		got := snapshotUnavailableMessage("resolve the repository root", wrapped, "")
		if strings.Contains(got, absoluteDir) {
			t.Fatalf("snapshotUnavailableMessage(%v) = %q, still contains the PathError's absolute path", wrapped, got)
		}
		if !strings.Contains(got, fs.ErrNotExist.Error()) {
			t.Errorf("snapshotUnavailableMessage(%v) = %q, want it to retain the errno-class reason %q", wrapped, got, fs.ErrNotExist.Error())
		}
	})

	t.Run("git ls-tree error naming the resolved repository root is stripped", func(t *testing.T) {
		err := fmt.Errorf("coach: git ls-tree failed for revision %q in %q: exit status 128: fatal: not a tree object", "deadbeef", absoluteDir)
		got := snapshotUnavailableMessage("read the HEAD snapshot", err, absoluteDir)
		if strings.Contains(got, absoluteDir) {
			t.Fatalf("snapshotUnavailableMessage(%q) = %q, still contains the absolute repository root", err, got)
		}
	})

	t.Run("snapshotListError whose wrapped git stderr embeds a known absolute path is stripped", func(t *testing.T) {
		// runGitBytesBoundedWith renders failures as "%s: %s" (wait error:
		// stderr), and git's own stderr text -- e.g. `fatal: cannot change
		// to '<dir>': No such file or directory` -- can itself embed the
		// repository root, independent of the wrapping fmt.Errorf. Unwrap()
		// must not be trusted to be path-free; the scrub loop has to run on
		// it too, not just in the default: fallback.
		gitErr := fmt.Errorf("exit status 128: fatal: cannot change to '%s': No such file or directory", absoluteDir)
		listErr := &snapshotListError{revision: "deadbeef", dir: absoluteDir, err: gitErr}
		got := snapshotUnavailableMessage("read the HEAD snapshot", listErr, absoluteDir)
		if strings.Contains(got, absoluteDir) {
			t.Fatalf("snapshotUnavailableMessage(%v) = %q, still contains the absolute repository root embedded in git's own stderr", listErr, got)
		}
	})

	t.Run("git ls-tree error whose %q-rendering needed escaping is stripped in both raw and escaped form", func(t *testing.T) {
		const quotableDir = `/tmp/coach-acceptance-repo-123"quote\dir`
		err := fmt.Errorf("coach: git ls-tree failed for revision %q in %q: exit status 128: fatal: not a tree object", "deadbeef", quotableDir)
		got := snapshotUnavailableMessage("read the HEAD snapshot", err, quotableDir)
		if strings.Contains(got, quotableDir) {
			t.Fatalf("snapshotUnavailableMessage(%q) = %q, still contains the raw absolute repository root", err, got)
		}
		quoted := strconv.Quote(quotableDir)
		escaped := quoted[1 : len(quoted)-1]
		if strings.Contains(got, escaped) {
			t.Fatalf("snapshotUnavailableMessage(%q) = %q, still contains the %%q-escaped absolute repository root %q", err, got, escaped)
		}
	})
}

func TestSuggestExitCodeFor(t *testing.T) {
	tests := []struct {
		code string
		want int
	}{
		{SuggestDiagInvalidArguments, 2},
		{SuggestDiagOutputInvalid, 2},
		{SuggestDiagOutputExists, 2},
		{SuggestDiagNoGoModules, 2},
		{SuggestDiagAmbiguousRoots, 2},
		{SuggestDiagIncomplete, 2},
		{SuggestDiagSnapshotUnavailable, 3},
		{SuggestDiagFailed, 3},
	}
	for _, tt := range tests {
		if got := suggestExitCodeFor(tt.code); got != tt.want {
			t.Errorf("suggestExitCodeFor(%q) = %d, want %d", tt.code, got, tt.want)
		}
	}
}

// TestSuggestProjectConfigIncompleteBudget locks the discovery → primary
// diagnostic → exit-2 glue for budget exhaustion through SuggestProjectConfig
// (issue #220). pkg/projectmodel covers incomplete discovery in isolation and
// TestSuggestPrimaryRootDiagnostic maps Complete=false in isolation; this is
// the package-level path that lowers suggestGoBudgets over a real multi-file
// Git snapshot so a regression in either hop fails here.
func TestSuggestProjectConfigIncompleteBudget(t *testing.T) {
	prev := suggestGoBudgets
	suggestGoBudgets = projectmodel.GoBudgets{MaxInputFiles: 1}
	t.Cleanup(func() { suggestGoBudgets = prev })

	dir := newTempGitRepoT(t)
	// Three go.mod files (mirrors pkg/projectmodel testdata/go_roots_incomplete)
	// so MaxInputFiles:1 is exceeded during the snapshot walk.
	for _, mod := range []string{"mod1", "mod2", "mod3"} {
		if err := os.MkdirAll(filepath.Join(dir, mod), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", mod, err)
		}
		commitFileT(t, dir, filepath.Join(mod, "go.mod"), "module example.com/"+mod+"\n\ngo 1.25\n")
	}

	const outRel = "suggested-project.json"
	result := SuggestProjectConfig(dir, outRel, true)

	if result.ExitCode != 2 {
		t.Fatalf("ExitCode = %d, want 2 (stderr envelope: %s)", result.ExitCode, result.Envelope)
	}
	if len(result.Candidate) != 0 {
		t.Fatalf("Candidate must be empty on incomplete discovery, got %q", result.Candidate)
	}

	var envelope struct {
		Diagnostics []struct {
			Code string `json:"code"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(result.Envelope, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v\nraw: %s", err, result.Envelope)
	}
	if len(envelope.Diagnostics) != 1 {
		t.Fatalf("diagnostics len = %d, want 1; envelope: %s", len(envelope.Diagnostics), result.Envelope)
	}
	if got := envelope.Diagnostics[0].Code; got != SuggestDiagIncomplete {
		t.Fatalf("primary diagnostic code = %q, want %q; envelope: %s", got, SuggestDiagIncomplete, result.Envelope)
	}

	if _, err := os.Stat(filepath.Join(dir, outRel)); !os.IsNotExist(err) {
		t.Fatalf("--output target %q must not be created on incomplete discovery; Stat err = %v", outRel, err)
	}
}
