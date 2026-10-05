package gitrepo

import (
	"bytes"
	"fmt"
	"strings"
)

// nameStatusRecord is one record from `git diff --name-status -z`: a status
// field, plus one path (A/M/D/other) or two paths (old, new for R/C).
type nameStatusRecord struct {
	status string
	paths  []string
}

// parseNameStatusZ parses the NUL-delimited output of
// `git diff --name-status -z`. It never invokes a shell and never
// interprets path bytes beyond splitting on NUL, so paths containing
// spaces, quotes, newlines, or non-ASCII bytes round-trip exactly.
func parseNameStatusZ(data []byte) ([]nameStatusRecord, error) {
	fields := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	if len(fields) == 1 && len(fields[0]) == 0 {
		return nil, nil
	}

	var records []nameStatusRecord
	for i := 0; i < len(fields); {
		status := string(fields[i])
		i++
		if status == "" {
			return nil, fmt.Errorf("malformed diff status stream: empty status field")
		}

		pathCount := 1
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			pathCount = 2
		}

		if i+pathCount > len(fields) {
			return nil, fmt.Errorf("malformed diff status stream: truncated record for status %q", status)
		}

		paths := make([]string, pathCount)
		for p := 0; p < pathCount; p++ {
			paths[p] = string(fields[i])
			i++
		}

		records = append(records, nameStatusRecord{status: status, paths: paths})
	}

	return records, nil
}
