package projectmodel

import (
	"context"
	"fmt"

	"io/fs"
)

// loadGoSnapshot discovers module roots, materializes snapshot once, and
// builds one SSA program per root. ctx cancellation stops further loads
// (loadStopped) without discarding roots already built. A non-nil error
// means the snapshot could not be materialized; per-root load failures are
// recorded on loadedGoRoot.loadErr instead.
func loadGoSnapshot(ctx context.Context, snapshot fs.FS, roots []string, budgets GoBudgets) (*loadedGoSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	discovery := discoverGoProject(snapshot, budgets)
	modules := discovery.Modules
	if len(roots) > 0 {
		modules, _ = filterToRoots(modules, discovery.Workspaces, roots)
	}
	loaded := &loadedGoSnapshot{
		cleanup:    func() {},
		discovery:  discovery,
		moduleDirs: mapKeysSorted(modules),
	}
	if ctx.Err() != nil {
		loaded.loadStopped = true
		return loaded, nil
	}

	tempDir, cleanup, err := materializeSnapshot(snapshot)
	if err != nil {
		return nil, fmt.Errorf("projectmodel: materializing snapshot for call-graph build: %w", err)
	}
	loaded.tempDir = tempDir
	loaded.cleanup = cleanup

	for _, mdir := range loaded.moduleDirs {
		if ctx.Err() != nil {
			loaded.loadStopped = true
			break
		}
		root := loadedGoRoot{dir: mdir}
		root.pkgs, root.prog, root.localPkgPaths, root.loadErr = loadGoSSAProgram(ctx, tempDir, mdir)
		loaded.roots = append(loaded.roots, root)
	}
	return loaded, nil
}
