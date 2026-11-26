package evaldetect

import (
	"fmt"
	"path"
	"strings"
)

// tsResolver maps a specifier to the root-relative TS file it names. It
// supports relative specifiers, tsconfig baseUrl and single-wildcard paths,
// and refuses the package.json forms it does not model.
type tsResolver struct {
	root         string
	config       tsCompilerPaths
	packageNames []string
}

func (r tsResolver) resolve(fromFile, specifier string, line int) (string, bool, error) {
	if err := r.refuse(fromFile, specifier, line); err != nil {
		return "", false, err
	}
	if isRelativeSpecifier(specifier) {
		target, ok := r.file(path.Join(path.Dir(fromFile), specifier), namesDirectory(specifier))
		return target, ok, nil
	}
	for _, candidate := range r.pathsCandidates(specifier) {
		if target, ok := r.file(candidate, false); ok {
			return target, true, nil
		}
	}
	if r.config.hasBaseURL {
		target, ok := r.file(path.Join(r.config.baseURL, specifier), namesDirectory(specifier))
		return target, ok, nil
	}
	return "", false, nil
}

// namesDirectory reports whether a specifier can only name a directory:
// ".", "..", or a path ending in "/" or in either of them. TS resolves such a
// specifier to the directory's index, never to a file beside it.
func namesDirectory(specifier string) bool {
	base := path.Base(specifier)
	return strings.HasSuffix(specifier, "/") || base == "." || base == ".."
}

func (r tsResolver) refuse(fromFile, specifier string, line int) error {
	if strings.HasPrefix(specifier, "#") {
		return fmt.Errorf("%w: %s:%d: %q is a package.json imports specifier", ErrRefused, fromFile, line, specifier)
	}
	for _, name := range r.packageNames {
		if specifier == name || strings.HasPrefix(specifier, name+"/") {
			return fmt.Errorf("%w: %s:%d: %q is a package.json self-reference", ErrRefused, fromFile, line, specifier)
		}
	}
	return nil
}
