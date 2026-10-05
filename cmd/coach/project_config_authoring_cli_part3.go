package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// rejectUnusableAuthoringOutput fails fast on --output shape or an existing
// target before the session. writeSuggestOutput's O_EXCL remains the sole
// existence authority against a concurrently created target.
func rejectUnusableAuthoringOutput(root string, f codesignalFlags) error {
	if !f.outputSet {
		return nil
	}
	clean, err := codesignalcli.ValidateAuthoringOutputPath(root, f.output)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(filepath.Join(root, clean)); err == nil {
		return fmt.Errorf("--output target already exists")
	}
	return nil
}

// tsRootDiscoverySnapshotUnavailable reports discovered's DiagTSRootUnavailable
// diagnostic, if present. DiscoverTSRoots' Complete field goes false for two
// distinct causes, and callers must not conflate them: DiagTSRootUnavailable
// means a read failure -- either the whole walk (Path ".", Roots/Candidates
// come back completely empty) or a single tsconfig.json/package.json the
// walk otherwise continued past (Path is that file, and Roots/Candidates may
// already hold real entries collected before the failure) -- while
// DiagTSRootIncomplete means mere budget truncation, where the partial list
// gathered so far is real data. Either DiagTSRootUnavailable case is treated
// as a hard failure here, matching the Go discovery family's own
// DiagRootUnavailable handling (any occurrence, whole-walk or single-file,
// maps to SuggestDiagSnapshotUnavailable): a read failure means some fact
// about the tree could not be established, so the roots collected around it
// are not trusted as a complete picture either. Only DiagTSRootIncomplete is
// safe to warn about and still show to the user.
func tsRootDiscoverySnapshotUnavailable(discovered projectmodel.TSRootDiscoveryResult) (projectmodel.Diagnostic, bool) {
	for _, diag := range discovered.Coverage.Diagnostics {
		if diag.Code == projectmodel.DiagTSRootUnavailable {
			return diag, true
		}
	}
	return projectmodel.Diagnostic{}, false
}

// runAuthorProjectConfigTypeScript dispatches `coach codesignal --baseline
// --suggest-project-config --project-language typescript`. The
// controlling-terminal check runs before any revision resolution, snapshot
// read, or discovery: without a controlling terminal on stdin, this function
// never prompts and never writes a policy file.
//
// Exit codes deliberately match the plain `--suggest-project-config` family's
// documented table (SuggestionResult/suggestExitCodeFor): 0 success, 2
// usage/discovery rejection (no controlling terminal, or a declined/
// cancelled/invalid guided-authoring outcome -- this dispatch's own
// interactive-decision equivalent of a discovery rejection), 3 for a failure
// resolving or reading the immutable revision/repository-root/snapshot this
// dispatch discovers TypeScript roots over, or for root discovery itself
// failing outright (as opposed to merely reporting an incomplete walk).
// What deliberately does NOT match: the report shape. This dispatch is
// interactive (it prompts over a real terminal), so its stderr is plain,
// human-facing text rather than the machine-readable NDJSON envelope
// `--suggest-project-config` writes, and an absolute invocation-directory
// path is acceptable in that text where it would not be in the envelope.
func runAuthorProjectConfigTypeScript(dir string, f codesignalFlags, stdout, stderr *os.File) int {
	if reason := interactiveRefusalReason(f, os.Stdin); reason != "" {
		fmt.Fprintf(stderr, "%s: %s; refusing to enter guided policy authoring or write a policy config. Draft the schema-1 project-config document yourself, have a human review and commit it, then rerun with --project-config <path>.\n", authorTSUsagePrefix, reason)
		return 2
	}
	return authorProjectConfigTypeScript(dir, f, os.Stdin, stdout, stderr)
}
