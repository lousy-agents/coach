package main

import (
	"os"
	"path/filepath"
)

type sigpathExcludingExecutablesS2 struct {
	dir      string
	excluded *bool
	names    []string
}

func (sigRecv *sigpathExcludingExecutablesS2) call() {

	for _, name := range sigRecv.names {
		if info, err := os.Stat(filepath.Join(sigRecv.dir, name)); err == nil && !info.IsDir() {
			*sigRecv.excluded = true
			break
		}
	}
}
