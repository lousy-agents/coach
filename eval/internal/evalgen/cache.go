package evalgen

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Location is where a cached program lives: Program is the tree an agent sees, Oracle holds tests/ and
// solution/, and Manifest is manifest.json.
type Location struct {
	Dir, Program, Oracle, Manifest string
}

// Store is the content-hash cache of generated programs, never inside the repository.
type Store struct {
	Root string
	// OracleRoot, when set, holds the hidden tests and the clean oracle apart from the agent-visible programs.
	OracleRoot string
}

func (s Store) locate(key string) Location {
	dir := filepath.Join(s.Root, key)
	oracle := filepath.Join(dir, "oracle")
	if s.OracleRoot != "" {
		oracle = filepath.Join(s.OracleRoot, key)
	}
	return Location{Dir: dir, Program: filepath.Join(dir, "program"), Oracle: oracle, Manifest: filepath.Join(dir, "manifest.json")}
}

// Lookup returns the cached program when its files still hash to the ids its manifest records.
func (s Store) Lookup(key string) (Location, Manifest, bool) {
	location := s.locate(key)
	var manifest Manifest
	data, err := os.ReadFile(location.Manifest)
	if err != nil || json.Unmarshal(data, &manifest) != nil {
		return location, manifest, false
	}
	tree, treeErr := readTree(location.Program)
	oracle, oracleErr := readOracle(location.Oracle)
	if treeErr != nil || oracleErr != nil {
		return location, manifest, false
	}
	intact := TreeID(tree) == manifest.Program.ProgramID && OracleID(oracle) == manifest.Template.OracleRef
	return location, manifest, intact
}

// Save writes a validated program, replacing whatever is cached under its key.
func (s Store) Save(key string, p Program) (Location, error) {
	location := s.locate(key)
	if err := os.MkdirAll(s.Root, 0o755); err != nil {
		return location, err
	}
	staging, err := os.MkdirTemp(s.Root, ".staging-")
	if err != nil {
		return location, err
	}
	defer os.RemoveAll(staging)
	manifest, _ := json.MarshalIndent(p.Manifest, "", "  ")
	if err := WriteTree(filepath.Join(staging, "program"), p.Tree); err != nil {
		return location, err
	}
	if err := os.WriteFile(filepath.Join(staging, "manifest.json"), append(manifest, '\n'), 0o644); err != nil {
		return location, err
	}
	if err := os.RemoveAll(location.Dir); err != nil {
		return location, err
	}
	if err := os.Rename(staging, location.Dir); err != nil {
		return location, err
	}
	return location, writeOracle(location.Oracle, p)
}

// oracleDirectories maps each part of the oracle to its directory under the oracle root.
func oracleDirectories(p *Program) map[string]*map[string][]byte {
	return map[string]*map[string][]byte{
		"tests": &p.Tests, "solution": &p.Solution,
		"followon/tests": &p.FollowOnTests, "followon/solution": &p.FollowOnSolution,
	}
}

func writeOracle(dir string, p Program) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for name, files := range oracleDirectories(&p) {
		if err := WriteTree(filepath.Join(dir, filepath.FromSlash(name)), *files); err != nil {
			return err
		}
	}
	return nil
}

func readOracle(dir string) (Program, error) {
	var oracle Program
	for name, files := range oracleDirectories(&oracle) {
		tree, err := readTree(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return Program{}, err
		}
		*files = tree
	}
	return oracle, nil
}
