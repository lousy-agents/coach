package evalpatterns

import (
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// CheckoutReference checks the entry, then clones clone into dir and checks
// out the entry's SHA through the census's isolated checkout, so neither the
// clone's nor the host's hooks, filters or configuration run (design §11).
// A clone that cannot be read or lacks the SHA is unusable input.
func CheckoutReference(reference Reference, clone, dir string) error {
	if err := reference.Check(); err != nil {
		return err
	}
	repository := evalcensus.Repository{ID: reference.Label, Path: clone, SHA: reference.SHA, Language: reference.Language}
	if err := evalcensus.CheckoutRepository(repository, dir); err != nil {
		reason := strings.TrimPrefix(err.Error(), "repository "+`"`+reference.Label+`": `)
		return reference.refuse(reason)
	}
	return nil
}
