package codesignal_test

import (
	"os"
	"path/filepath"
	"regexp"
)

type sigscanForGuardedSymbols31948627 struct {
	hits     map[string][]string
	patterns []*regexp.
			Regexp
	symbols []string
}

func (sigRecv *sigscanForGuardedSymbols31948627) call(path string, entry os.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for i, symbol := range sigRecv.symbols {
		if sigRecv.patterns[i].Match(content) {
			sigRecv.hits[path] = append(sigRecv.hits[path], symbol)
		}
	}
	return nil
}
