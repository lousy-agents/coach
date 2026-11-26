package evaldetect

import (
	"path"
	"strings"
)

// importPathOf maps a root-relative directory to its import path in the
// innermost module containing it.
func importPathOf(dir string, modules []goModule) (string, bool) {
	best := -1
	for i, module := range modules {
		if module.dir != "." && dir != module.dir && !strings.HasPrefix(dir, module.dir+"/") {
			continue
		}
		if best < 0 || len(module.dir) > len(modules[best].dir) {
			best = i
		}
	}
	if best < 0 {
		return "", false
	}
	module := modules[best]
	rest := strings.TrimPrefix(strings.TrimPrefix(dir, module.dir), "/")
	if module.dir == "." {
		rest = strings.TrimPrefix(dir, ".")
	}
	return path.Join(module.path, rest), true
}
