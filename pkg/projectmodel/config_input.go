package projectmodel

import "path"

// IsConfigInput reports whether p, a slash-separated repository path, names a
// file a project model reads as configuration rather than as source: a Go
// manifest (go.mod, go.work), or a TypeScript one (package.json and every
// tsconfig*.json, so an extends target is included). It is the one list a
// caller consults to know which uncommitted files change what a model sees, so
// a file a model starts reading is added here and in the model together.
func IsConfigInput(p string) bool {
	base := path.Base(p)
	return isGoManifest(base) || isTSSidecarConfigFile(base)
}

// isGoManifest reports whether base is a manifest Go root discovery reads.
func isGoManifest(base string) bool {
	return base == "go.mod" || base == "go.work"
}
