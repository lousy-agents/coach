package codesignalcli

import (
	"bytes"
	"sort"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// tallyClassified splits classified (files already labeled by
// classifySourceFiles) into files that ship (kept) and files that don't
// (excluded), grouped by (SourceScope reason, Language) pair. It is shared
// by ApplySourceScope and ApplyBaselineSourceScope, whose only difference is
// what they do with the two results.
func tallyClassified(classified []gitrepo.SelectedFile) (kept []gitrepo.SelectedFile, excluded []codesignal.CoverageGroup) {
	type groupKey struct{ reason, language string }
	counts := make(map[groupKey]int)

	kept = make([]gitrepo.SelectedFile, 0, len(classified))
	for _, file := range classified {
		if file.SourceScope == SourceScopeTestOnly || file.SourceScope == SourceScopeExcluded {
			counts[groupKey{reason: file.SourceScope, language: string(file.Language)}]++
			continue
		}
		kept = append(kept, file)
	}

	keys := make([]groupKey, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].reason != keys[j].reason {
			return keys[i].reason < keys[j].reason
		}
		return keys[i].language < keys[j].language
	})

	for _, key := range keys {
		excluded = append(excluded, codesignal.CoverageGroup{
			Reason:   key.reason,
			Language: key.language,
			Count:    counts[key],
		})
	}

	return kept, excluded
}

// stripJSONCComments strips // and /* */ outside strings, then trailing commas.
// Unterminated /* returns the original bytes so Unmarshal fails closed.
func stripJSONCComments(data []byte) []byte {
	var out bytes.Buffer
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if inString {
			inString, escaped = advanceInsideJSONString(&out, b, escaped)
			continue
		}
		switch {
		case b == '"':
			inString = true
			out.WriteByte(b)
		case b == '/' && i+1 < len(data) && data[i+1] == '/':
			i = skipJSONCLineComment(data, i)
			if i < len(data) {
				out.WriteByte('\n')
			}
		case b == '/' && i+1 < len(data) && data[i+1] == '*':
			next, ok := skipJSONCBlockComment(data, i)
			if !ok {
				return data
			}
			i = next
		default:
			out.WriteByte(b)
		}
	}
	return stripTrailingCommas(out.Bytes())
}
