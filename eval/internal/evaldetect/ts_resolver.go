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
		return r.file(path.Join(path.Dir(fromFile), specifier))
	}
	for _, candidate := range r.pathsCandidates(specifier) {
		if target, ok, err := r.file(candidate); ok || err != nil {
			return target, ok, err
		}
	}
	if r.config.hasBaseURL {
		return r.file(path.Join(r.config.baseURL, specifier))
	}
	return "", false, nil
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
