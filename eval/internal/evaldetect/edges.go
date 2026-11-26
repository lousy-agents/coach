package evaldetect

import "sort"

// importEdge is one resolved import between two repository directories.
type importEdge struct {
	fromFile string
	fromDir  string
	toDir    string
	line     int
}

func (p Policy) violations(edges []importEdge) []Finding {
	var findings []Finding
	for _, edge := range edges {
		if !p.forbids(edge.fromDir, edge.toDir) {
			continue
		}
		findings = append(findings, Finding{
			RuleID:  layerViolation,
			Path:    edge.fromFile,
			Subject: layerViolation + ":" + edge.fromDir + "->" + edge.toDir,
			Line:    edge.line,
		})
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Line < findings[j].Line
	})
	return findings
}
