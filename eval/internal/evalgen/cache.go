package evalgen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// DefaultStore is $XDG_CACHE_HOME/coach-eval/fixtures.
func DefaultStore() Store {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return Store{Root: filepath.Join(base, "coach-eval", "fixtures")}
}

// Key hashes everything that decides the emitted bytes.
func Key(language Language, cell Cell, seed int64, panel string) string {
	cellJSON, _ := json.Marshal(cell)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d|%s|operators=", language, language.Version(), cellJSON, seed, panel)))
	return hex.EncodeToString(sum[:])
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
	tests, testsErr := readTree(filepath.Join(location.Oracle, "tests"))
	solution, solutionErr := readTree(filepath.Join(location.Oracle, "solution"))
	if treeErr != nil || testsErr != nil || solutionErr != nil {
		return location, manifest, false
	}
	intact := TreeID(tree) == manifest.Program.ProgramID && OracleID(Program{Tests: tests, Solution: solution}) == manifest.Template.OracleRef
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

func writeOracle(dir string, p Program) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := WriteTree(filepath.Join(dir, "tests"), p.Tests); err != nil {
		return err
	}
	return WriteTree(filepath.Join(dir, "solution"), p.Solution)
}
