package codesignalcli

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// hunkHeaderPattern matches a unified diff hunk header:
// "@@ -oldStart[,oldCount] +newStart[,newCount] @@" (trailing section
// heading ignored).
var hunkHeaderPattern = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func parseChangedRanges(diff []byte) ([]codesignal.LineRange, error) {
	var ranges []codesignal.LineRange

	scanner := bufio.NewScanner(bytes.NewReader(diff))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}

		r, ok, err := parseHunkHeaderRange(line)
		if err != nil {
			return nil, err
		}
		if ok {
			ranges = append(ranges, r)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ranges, nil
}

func parseHunkHeaderRange(line string) (r codesignal.LineRange, ok bool, err error) {
	match := hunkHeaderPattern.FindStringSubmatch(line)
	if match == nil {
		return codesignal.LineRange{}, false, fmt.Errorf("unparsable hunk header: %q", line)
	}

	newStart, err := strconv.Atoi(match[1])
	if err != nil {
		return codesignal.LineRange{}, false, fmt.Errorf("invalid hunk new-start in %q: %w", line, err)
	}

	newCount := 1
	if match[2] != "" {
		newCount, err = strconv.Atoi(match[2])
		if err != nil {
			return codesignal.LineRange{}, false, fmt.Errorf("invalid hunk new-count in %q: %w", line, err)
		}
	}

	if newCount == 0 {
		return codesignal.LineRange{}, false, nil
	}

	return codesignal.LineRange{
		StartRow: uint(newStart - 1),
		EndRow:   uint(newStart + newCount - 2),
	}, true, nil
}
