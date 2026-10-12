package domain

import (
	"sort"
)

// File is one source file recorded in a Model.
type File struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Language    string `json:"language"`
	BlobHash    string `json:"blob_hash,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
}

func canonicalFiles(in []File) []File {
	if len(in) == 0 {
		return in
	}
	out := append([]File(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.BlobHash != b.BlobHash {
			return a.BlobHash < b.BlobHash
		}
		if a.ContentHash != b.ContentHash {
			return a.ContentHash < b.ContentHash
		}
		return a.Language < b.Language
	})
	return out
}
