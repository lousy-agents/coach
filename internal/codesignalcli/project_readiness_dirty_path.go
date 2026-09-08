package codesignalcli

import (
	"path"
	"strings"
)

var alwaysRelevantMetadataBasenames = map[string]bool{
	"package.json":        true,
	"package-lock.json":   true,
	"yarn.lock":           true,
	"pnpm-lock.yaml":      true,
	"npm-shrinkwrap.json": true,
	"bun.lockb":           true,
}

func isRelevantDirtyPath(candidate string, roots []string, policyPath string) bool {
	if candidate == policyPath {
		return true
	}
	base := path.Base(candidate)
	if strings.HasPrefix(base, "tsconfig") {
		return true
	}
	if alwaysRelevantMetadataBasenames[base] {
		return true
	}
	for _, root := range roots {
		if pathUnderRoot(candidate, root) {
			return true
		}
	}
	return false
}

func pathUnderRoot(candidate, root string) bool {
	if root == "." || root == "" {
		return true
	}
	return candidate == root || strings.HasPrefix(candidate, root+"/")
}
