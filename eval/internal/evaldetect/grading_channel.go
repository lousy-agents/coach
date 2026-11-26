package evaldetect

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GradingChannelRule is the rule ID of a reference to the grading channel from non-test code.
const GradingChannelRule = "gaming.grading_channel_access"

// GradingChannelAccess reports non-test Go and TypeScript code under root that reaches for the grading channel: an
// inherited descriptor (os.NewFile in Go, an fs call on a descriptor number of 3 or more in TypeScript),
// process.binding, or a /proc path to a descriptor or to memory. Test files, dependencies and hidden directories are
// not read. It refuses, as the layer graph does, a file it cannot read exactly, so that nothing is skipped silently.
func GradingChannelAccess(root string) ([]Finding, error) {
	var findings []Finding
	err := walkFiles(root, isChannelSource, func(rel string) error {
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		found, err := channelFindingsIn(rel, string(source))
		findings = append(findings, found...)
		return err
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Line < findings[j].Line
	})
	return findings, nil
}

func isChannelSource(name string) bool {
	if strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") || strings.HasSuffix(name, "_test.go") {
		return false
	}
	return strings.HasSuffix(name, ".go") || isTSSource(name)
}

func channelFindingsIn(rel, source string) ([]Finding, error) {
	if strings.HasSuffix(rel, ".go") {
		return goChannelFindings(rel, source)
	}
	return tsChannelFindings(rel, source)
}

func channelFinding(rel, pattern string, line int) Finding {
	return Finding{RuleID: GradingChannelRule, Path: rel, Subject: GradingChannelRule + ":" + pattern, Line: line}
}
