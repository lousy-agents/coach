package evalgen

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

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
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return Program{}, err
		}
		*files = tree
	}
	return oracle, nil
}
