package main

import (
	"strings"
)

func stracePaths(rest string) []string {
	seen := map[string]struct{}{}
	var paths []string
	add := func(p string) {
		p = unescapeC(p)
		if p == "" || !strings.HasPrefix(p, "/") {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	for _, q := range straceQuoted.FindAllStringSubmatch(rest, -1) {
		add(q[1])
	}
	for _, q := range straceFdPath.FindAllStringSubmatch(rest, -1) {
		add(q[1])
	}
	return paths
}

func execveArgv(rest string) []string {
	start := strings.Index(rest, "[")
	end := strings.Index(rest, "]")
	if start < 0 || end <= start {
		return unescapeStraceQuoted(rest)
	}
	return unescapeStraceQuoted(rest[start : end+1])
}

func unescapeStraceQuoted(s string) []string {
	var out []string
	for _, q := range straceQuoted.FindAllStringSubmatch(s, -1) {
		out = append(out, unescapeC(q[1]))
	}
	return out
}

func unescapeC(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '"', '\\':
				b.WriteByte(s[i])
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
