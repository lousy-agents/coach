package main

import (
	"os"
	"path/filepath"
	"strings"
)

func linuxPathAllowed(allow linuxAllowlist, path string) bool {
	clean := filepath.Clean(path)
	for _, exact := range allow.exact {
		if clean == filepath.Clean(exact) {
			return true
		}
	}
	for _, prefix := range allow.prefixes {
		p := filepath.Clean(prefix)
		if clean == p || strings.HasPrefix(clean, p+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func linuxAncestorExact(allow linuxAllowlist, path string) bool {
	clean := filepath.Clean(path)
	for _, ancestor := range allow.ancestors {
		if clean == filepath.Clean(ancestor) {
			return true
		}
	}
	return false
}

func linuxProbeAllowed(allow linuxAllowlist, syscall, path string) bool {
	if linuxPathAllowed(allow, path) {
		return true
	}
	if _, meta := linuxMetadataSyscalls[syscall]; meta && linuxAncestorExact(allow, path) {
		return true
	}
	return false
}
