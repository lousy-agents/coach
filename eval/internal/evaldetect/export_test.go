package evaldetect

// ScanWithoutVersionCheck runs a stand-in gocognit binary, which carries no
// gocognit build info and so would fail the version check first.
func (g Gocognit) ScanWithoutVersionCheck(root string) ([]Finding, error) {
	return g.scan(root)
}

var ResolveGocognitPinnedTo = resolveGocognit

// SetMaxCallGraphNodes lowers the call-graph limit until restore is called.
func SetMaxCallGraphNodes(n int) (restore func()) {
	saved := maxCallGraphNodes
	maxCallGraphNodes = n
	return func() { maxCallGraphNodes = saved }
}
