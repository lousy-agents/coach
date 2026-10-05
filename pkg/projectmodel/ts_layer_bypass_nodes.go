package projectmodel

import (
	"path"
	"strings"
)

func tsLayerBypassNodePositions(facts []CallFact) map[string]layerBypassNodePosition {
	set := &nodePositionSet{positions: map[string]layerBypassNodePosition{}}
	for _, f := range facts {
		set.addPair(f.From, f.To)
	}
	return set.positions
}

type nodePositionSet struct {
	positions map[string]layerBypassNodePosition
}

func (s *nodePositionSet) addPair(from, to string) {
	for _, nodeID := range [2]string{from, to} {
		if _, ok := s.positions[nodeID]; ok {
			continue
		}
		if pos, ok := tsLayerBypassNodePosition(nodeID); ok {
			s.positions[nodeID] = pos
		}
	}
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

func tsLayerBypassLayerMatchesFiles(files []File, layer BypassLayer) bool {
	for _, f := range files {
		if layerBypassContainsDir(layer, path.Dir(f.Path)) {
			return true
		}
	}
	return false
}
