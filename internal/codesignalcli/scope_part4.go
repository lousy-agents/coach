package codesignalcli

import (
	"bytes"

	"fmt"

	"path/filepath"

	"strings"
)

// snapshotBuildTarget preserves the meaning of relative package patterns
// supplied from a subdirectory while making them point at the HEAD snapshot.
func snapshotBuildTarget(target, repositoryRoot, invocationDir, snapshotDir string) (string, error) {
	resolvedInvocationDir, err := filepath.EvalSymlinks(invocationDir)
	if err != nil {
		return "", fmt.Errorf("resolving invocation directory: %w", err)
	}
	if filepath.IsAbs(target) {
		resolvedTarget, err := filepath.EvalSymlinks(target)
		if err != nil {
			return "", fmt.Errorf("resolving build target: %w", err)
		}
		rel, err := filepath.Rel(repositoryRoot, resolvedTarget)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("build target %q is outside the repository", target)
		}
		return filepath.Join(snapshotDir, rel), nil
	}
	if !strings.HasPrefix(target, ".") {
		return target, nil
	}
	relDir, err := filepath.Rel(repositoryRoot, resolvedInvocationDir)
	if err != nil {
		return "", fmt.Errorf("resolving build target: %w", err)
	}
	return filepath.Join(snapshotDir, relDir, target), nil
}
func stripTrailingCommas(data []byte) []byte {
	var out bytes.Buffer
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if inString {
			inString, escaped = advanceInsideJSONString(&out, b, escaped)
			continue
		}
		if b == '"' {
			inString = true
			out.WriteByte(b)
			continue
		}
		if b == ',' && trailingCommaFollowedByClose(data, i) {
			continue
		}
		out.WriteByte(b)
	}
	return out.Bytes()
}
