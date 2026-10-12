package projectmodel

// layerBypassNodePosition is one local function's resolved declaration
// position: Dir drives RequiredLayer classification (see
// layerBypassContainsDir), File/Line are the repository-relative position
// LayerBypassStep.Path/Line carry into LayerBypassWitness.Path.
type layerBypassNodePosition struct {
	Dir  string
	File string
	Line int
}

// stepPathFullyClassified reports whether every non-sink node on stepPath
// (i.e. every node but the last) has an entry in nodePositions. A node with
// no entry has an unresolvable package directory (see fnPosition), so its
// required-layer membership was never evaluated; the caller must treat that
// as ambiguous rather than assume the node is outside RequiredLayer.
func stepPathFullyClassified(stepPath []ReachabilityStep, nodePositions map[string]layerBypassNodePosition) bool {
	if len(stepPath) == 0 {
		return true
	}
	for _, step := range stepPath[:len(stepPath)-1] {
		if _, ok := nodePositions[step.NodeID]; !ok {
			return false
		}
	}
	return true
}

// layerBypassSteps converts stepPath (reconstructReachabilityPath's shared
// ReachabilityStep shape) into LayerBypassSteps, filling Path/Line from
// nodePositions for whichever nodes resolved a position -- leaving them
// zero-valued for the rest (typically only the sink).
func layerBypassSteps(stepPath []ReachabilityStep, nodePositions map[string]layerBypassNodePosition) []LayerBypassStep {
	steps := make([]LayerBypassStep, len(stepPath))
	for i, step := range stepPath {
		ls := LayerBypassStep{NodeID: step.NodeID}
		if pos, ok := nodePositions[step.NodeID]; ok {
			ls.Path = pos.File
			ls.Line = pos.Line
		}
		steps[i] = ls
	}
	return steps
}
