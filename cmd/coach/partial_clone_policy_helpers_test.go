package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/gomega"
)

func putPolicyOnDisk(clone string) {
	ExpectWithOffset(1, os.MkdirAll(filepath.Join(clone, "cfg"), 0o755)).To(Succeed())
	ExpectWithOffset(1, os.WriteFile(filepath.Join(clone, "cfg", "p.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
}
