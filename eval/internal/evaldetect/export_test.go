package evaldetect

// ScanWithoutVersionCheck runs a stand-in gocognit binary, which carries no
// gocognit build info and so would fail the version check first.
func (g Gocognit) ScanWithoutVersionCheck(root string) ([]Finding, error) {
	return g.scan(root)
}

var ResolveGocognitPinnedTo = resolveGocognit
