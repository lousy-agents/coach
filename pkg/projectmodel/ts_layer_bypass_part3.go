package projectmodel

import (
	"context"

	"io/fs"
	"path"

	"strings"
)

// Deprecated: call BuildTypeScriptModelViaSidecar once and pass the Model to
// BuildTypeScriptLayerBypassFromModel / BuildTypeScriptReachabilityFromModel
// instead, so multiple derivations share one sidecar round trip.
func BuildTypeScriptLayerBypass(ctx context.Context, snapshot fs.FS, meta SnapshotMeta, opts TSSidecarOptions, requiredLayer BypassLayer) (LayerBypassResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	model, err := BuildTypeScriptModelViaSidecar(ctx, snapshot, meta, opts)
	if err != nil {
		return LayerBypassResult{}, err
	}

	return BuildTypeScriptLayerBypassFromModel(ctx, model, requiredLayer), nil
}
func tsLayerBypassLayerMatchesFiles(files []File, layer BypassLayer) bool {
	for _, f := range files {
		if layerBypassContainsDir(layer, path.Dir(f.Path)) {
			return true
		}
	}
	return false
}

// tsLayerBypassRequiredLayerNodes decides the ambiguous-layer match from
// snapshotFiles, not from the returned node-position map: the real
// sidecar's call graph only ever contains route-handler-to-sink edges, so a
// required layer with no route handler of its own -- the exact shape a
// genuine layer bypass produces -- would never appear as a CallFact
// endpoint even though real files live there.
func tsLayerBypassRequiredLayerNodes(snapshotFiles []File, callGraphNodePositions map[string]layerBypassNodePosition, requiredLayer BypassLayer) (map[string]bool, bool) {
	requiredLayerNodes := map[string]bool{}
	for node, pos := range callGraphNodePositions {
		if layerBypassContainsDir(requiredLayer, pos.Dir) {
			requiredLayerNodes[node] = true
		}
	}
	ambiguousLayer := len(requiredLayer.Prefixes) == 0 || !tsLayerBypassLayerMatchesFiles(snapshotFiles, requiredLayer)
	return requiredLayerNodes, ambiguousLayer
}

// tsLayerBypassNodePosition parses a TS call-graph node ID of the
// "file:<repo-relative path>#<name>" shape (reachability.ts's
// functionSourceId). It reports false for a synthetic sink node ID (e.g.
// "(PrismaClient).findMany"), which carries no file path at all.
func tsLayerBypassNodePosition(nodeID string) (layerBypassNodePosition, bool) {
	rest, ok := strings.CutPrefix(nodeID, "file:")
	if !ok {
		return layerBypassNodePosition{}, false
	}
	filePath, _, ok := strings.Cut(rest, "#")
	if !ok || filePath == "" {
		return layerBypassNodePosition{}, false
	}
	return layerBypassNodePosition{Dir: path.Dir(filePath), File: filePath}, true
}
