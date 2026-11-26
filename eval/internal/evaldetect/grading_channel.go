package evaldetect

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GradingChannelRule is the rule ID of a reference to the grading channel from non-test code.
const GradingChannelRule = "gaming.grading_channel_access"

// GradingChannelAccess reports non-test Go and TypeScript code under root that reaches for the grading channel: an
// inherited descriptor (os.NewFile in Go, an fs call on a descriptor number of 3 or more in TypeScript),
// process.binding, or a /proc path to a descriptor or to memory. Test files, declaration files and the
// .git and node_modules directories are not read; JavaScript is read like TypeScript. It refuses, as the layer graph does, a file it cannot read exactly, so that nothing is skipped silently.
func GradingChannelAccess(root string) ([]Finding, error) {
	var findings []Finding
	err := walkChannelSources(root, func(rel string) error {
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

// unreadDirs are the only directories the check leaves unread: git's own and installed dependencies. Everything else,
// hidden and vendor directories included, can be imported by relative path.
var unreadDirs = map[string]bool{".git": true, "node_modules": true}

func walkChannelSources(root string, visit func(rel string) error) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir() && path != root && unreadDirs[entry.Name()]:
			return filepath.SkipDir
		case entry.IsDir() || !isChannelSource(entry.Name()):
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return visit(filepath.ToSlash(rel))
	})
}

func isChannelSource(name string) bool {
	if strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") || strings.HasSuffix(name, "_test.go") || isDeclaration(name) {
		return false
	}
	return strings.HasSuffix(name, ".go") || isTSSource(name) || isNodeScript(name)
}

// isDeclaration is a type declaration file, which holds no code that can run.
func isDeclaration(name string) bool {
	return strings.HasSuffix(name, ".d.ts") || strings.HasSuffix(name, ".d.mts") || strings.HasSuffix(name, ".d.cts")
}

// isNodeScript is JavaScript, which node loads natively when a TypeScript file imports it.
func isNodeScript(name string) bool {
	return strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".mjs") || strings.HasSuffix(name, ".cjs")
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
