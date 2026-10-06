package tstestutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
)

// AllowedAnalysisNodeMajors is the set of Node majors
// prependAllowedAnalysisNodeToPath treats as already acceptable on the host
// PATH. It must be kept equal to codesignalcli.SupportedNodeMajors --
// tstestutil cannot import codesignalcli directly without an import cycle,
// so cmd/coach's TestTSTestutilAllowedNodeMajorsMatchesSupportedNodeMajors
// binds the two together.
var AllowedAnalysisNodeMajors = []int{24, 26}

func allowedAnalysisNodeMajor(major int) bool {
	for _, allowed := range AllowedAnalysisNodeMajors {
		if allowed == major {
			return true
		}
	}
	return false
}

func hostNodeMajorOnPath() (int, bool) {
	path, err := exec.LookPath("node")
	if err != nil {
		return 0, false
	}
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return 0, false
	}
	trimmed := strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
	majorPart, _, _ := strings.Cut(trimmed, ".")
	major, err := strconv.Atoi(majorPart)
	if err != nil {
		return 0, false
	}
	return major, true
}

func prependAllowedAnalysisNodeToPath() {
	if major, ok := hostNodeMajorOnPath(); ok && allowedAnalysisNodeMajor(major) {
		return
	}
	bin := filepath.Join(os.Getenv("HOME"), ".local", "share", "mise", "installs", "node", "24", "bin")
	if _, err := os.Stat(filepath.Join(bin, "node")); err != nil {
		return
	}
	GinkgoT().Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
