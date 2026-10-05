// Package projectconfig owns the project-policy document (project.json): its
// frozen schema-1 shape, strict decoding and validation, and the bounded read
// of the committed document at an analyzed revision.
package projectconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Project-config boundary budgets. Config is repository-controlled input and
// must fail closed before unbounded memory/CPU or a hung git child can stall
// the CLI.
const (
	MaxBytes     = 1 << 20 // 1 MiB
	maxJSONDepth = 32
	MaxGitStderr = 64 << 10 // 64 KiB
	GitTimeout   = 30 * time.Second
	// MaxLayerPrefixes bounds the sorted prefix-overlap scan so a
	// hostile but still ≤1 MiB config cannot force quadratic validation CPU.
	MaxLayerPrefixes = 4096
	// MaxRoots bounds the declared roots list. Unlike layer
	// prefixes (a pure in-process string-comparison budget), each declared
	// root can drive up to three git child-process spawns in
	// checkProjectShape's non-root package.json probe
	// (projectcheck/project_shape.go), so this budget must stay small enough that
	// even the worst case (no package.json under any root) completes in a
	// few seconds rather than fanning out into tens of thousands of git
	// invocations from a config that is still well under
	// MaxBytes.
	MaxRoots = 256
)

type Config struct {
	SchemaVersion    string            `json:"schema_version"`
	Roots            []string          `json:"roots"`
	Layers           []Layer           `json:"layers,omitempty"`
	ForbiddenImports []ForbiddenImport `json:"forbidden_imports,omitempty"`
	SourceSinkPack   string            `json:"source_sink_pack,omitempty"`
	RequiredLayer    string            `json:"required_layer,omitempty"`
}

type Layer struct {
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes"`
}

type ForbiddenImport struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Digest returns a stable hex digest of validated project-config bytes.
func Digest(config json.RawMessage) string {
	sum := sha256.Sum256(config)
	return "pcfg_" + hex.EncodeToString(sum[:])
}

func validateJSON(data []byte) error {
	_, err := Parse(data)
	return err
}

// Parse performs validateJSON's full decode and
// schema validation, additionally returning the decoded Config. It
// exists so a caller that needs the decoded value (checkPolicy, via
// LoadForReadiness) never has to run a second, redundant decode
// of bytes validateJSON already accepted.
func Parse(data []byte) (Config, error) {
	config, err := decode(data)
	if err != nil {
		return Config{}, err
	}
	if err := validateRoots(config.Roots); err != nil {
		return Config{}, err
	}
	seenLayerNames, err := validateLayers(config.Layers)
	if err != nil {
		return Config{}, err
	}
	if err := validateForbiddenImports(config.ForbiddenImports, seenLayerNames); err != nil {
		return Config{}, err
	}
	if err := validateLayerReferences(config, seenLayerNames); err != nil {
		return Config{}, err
	}
	return config, nil
}

const (
	DefaultPath = "project.json"
)
