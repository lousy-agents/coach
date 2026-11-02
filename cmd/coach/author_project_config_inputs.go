package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/configauthoring"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// rejectUnusableAuthoringOutput fails fast on --output shape or an existing
// target before the session. When --suggest-project-config omits --output,
// the implicit .lousy-agents/coach.json destination is checked the same way
// once a non-mutating Lstat shows .lousy-agents is a real directory. A
// missing directory is left for the approved write to create; a symlink or
// non-directory parent is refused here and is neither created nor followed.
// writeSuggestOutput's O_EXCL remains the sole existence authority against a
// concurrently created target.
func rejectUnusableAuthoringOutput(root string, f codesignalFlags) error {
	if !f.outputSet {
		if !f.suggestProjectConfig {
			return nil
		}
		return rejectImplicitDefaultAuthoringTarget(root)
	}
	clean, err := configauthoring.ValidateAuthoringOutputPath(root, f.output)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(filepath.Join(root, clean)); err == nil {
		return fmt.Errorf("--output target already exists")
	}
	return nil
}

// rejectImplicitDefaultAuthoringTarget inspects the implicit
// .lousy-agents/coach.json destination without creating it. The file is
// Lstat'd only after the parent Lstat shows a real directory, so a symlink
// at .lousy-agents is not followed.
func rejectImplicitDefaultAuthoringTarget(root string) error {
	parentName := filepath.Dir(configauthoring.DefaultSuggestConfigRel)
	dir := filepath.Join(root, parentName)
	info, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("parent path component %q is a symlink", parentName)
	}
	if !info.IsDir() {
		return fmt.Errorf("parent path component %q is not a directory", parentName)
	}
	target := filepath.Join(dir, filepath.Base(configauthoring.DefaultSuggestConfigRel))
	if _, err := os.Lstat(target); err == nil {
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
