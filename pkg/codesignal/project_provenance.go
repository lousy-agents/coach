package codesignal

// ProjectProvenance records how a TypeScript project report was produced:
// which analyzer ran, which runtime was used, which package manager was
// detected, and what coverage each revision received. It is optional on
// Report and populated only for schema-2 TypeScript reports (Go reports
// omit it via omitempty).
type ProjectProvenance struct {
	Language       string                    `json:"language"`
	ConfigDigest   string                    `json:"config_digest"`
	SelectedRoots  []string                  `json:"selected_roots"`
	Analyzer       ProvenanceAnalyzer        `json:"analyzer"`
	Runtime        ProvenanceRuntime         `json:"runtime"`
	PackageManager *ProvenancePackageManager `json:"package_manager,omitempty"`
	Head           ProvenanceRevision        `json:"head"`
	Base           *ProvenanceRevision       `json:"base,omitempty"`
}

// ProvenanceAnalyzer identifies the analyzer binary that produced the report.
type ProvenanceAnalyzer struct {
	Version         string `json:"version"`
	Digest          string `json:"digest"`
	ProtocolVersion int    `json:"protocol_version"`
}

// ProvenanceRuntime describes the Node/Bun runtime resolved for the project.
// DeclaredVersion is populated only when the project pins a version (T13/#354).
type ProvenanceRuntime struct {
	Kind            string `json:"kind"`
	Version         string `json:"version"`
	Origin          string `json:"origin"`
	CompilerVersion string `json:"compiler_version"`
	CompilerOrigin  string `json:"compiler_origin"`
	DeclaredVersion string `json:"declared_version,omitempty"`
}

// ProvenancePackageManager describes the package manager detected from
// snapshot metadata. The whole field is omitempty when no supported manager
// is identified; Version is omitempty when it cannot be verified from the
// lockfile.
type ProvenancePackageManager struct {
	Kind    string `json:"kind"`
	Version string `json:"version,omitempty"`
	Origin  string `json:"origin"`
}

// ProvenanceRevision records coverage for one revision (head or base) of a
// TypeScript project report.
type ProvenanceRevision struct {
	Revision string             `json:"revision"`
	Coverage ProvenanceCoverage `json:"coverage"`
}

// ProvenanceCoverage holds the coverage phase outcome for each analysis
// dimension. Values: complete, incomplete, not_run, not_requested.
type ProvenanceCoverage struct {
	Model        string `json:"model"`
	Bypass       string `json:"bypass"`
	Reachability string `json:"reachability"`
}
