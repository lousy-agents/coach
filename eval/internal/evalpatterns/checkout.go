package evalpatterns

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// urlScheme matches a clone given as a URL, which git would read through a
// transport rather than from the local disk.
var urlScheme = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9+.-]*)://`)

// CheckoutReference checks the entry, then clones clone into dir and checks
// out the entry's SHA through the census's isolated checkout, so neither the
// clone's nor the host's hooks, filters or configuration run (design §11).
// The clone must be a local directory: it is passed to git as an absolute
// path, so git never reads it through a network or ssh transport. A clone
// that cannot be read or lacks the SHA is unusable input.
func CheckoutReference(reference Reference, clone, dir string) error {
	if err := reference.Check(); err != nil {
		return err
	}
	local, err := localClone(clone)
	if err != nil {
		return reference.refuse(err.Error())
	}
	repository := evalcensus.Repository{ID: reference.Label, Path: local, SHA: reference.SHA, Language: reference.Language}
	if err := evalcensus.CheckoutRepository(repository, dir); err != nil {
		reason := strings.TrimPrefix(err.Error(), "repository "+`"`+reference.Label+`": `)
		return reference.refuse(reason)
	}
	return nil
}

// localClone is clone as an absolute path to an existing directory, or an
// error naming why it is not one.
func localClone(clone string) (string, error) {
	if scheme := urlScheme.FindStringSubmatch(clone); scheme != nil {
		return "", fmt.Errorf("clone %q is a URL (scheme %q); give the path of a local clone", clone, scheme[1])
	}
	local, err := filepath.Abs(clone)
	if err != nil {
		return "", fmt.Errorf("clone %q is not a local directory: %v", clone, err)
	}
	if info, err := os.Stat(local); err != nil || !info.IsDir() {
		return "", fmt.Errorf("clone %q is not a local directory", clone)
	}
	return local, nil
}
