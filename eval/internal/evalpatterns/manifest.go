package evalpatterns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// Manifest is the committed list of reference repositories. The clone's
// path is not part of it: it is given on the command line, so no path or
// repository name is committed.
type Manifest struct {
	SchemaVersion string      `json:"schema_version"`
	References    []Reference `json:"references"`
}

// Reference is one reference repository: an owner-chosen label, the commit
// it is read at, its language and its licence.
type Reference struct {
	Label    string `json:"label"`
	SHA      string `json:"sha"`
	Language string `json:"language"`
	Licence  string `json:"licence"`
}

// labelPattern keeps a label usable as one directory name under
// eval/fixtures/antipatterns/.
var labelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// maxLabelLength is the profile schema's cap on a label, enforced when the
// manifest loads so an entry is never read only to fail validation after.
const maxLabelLength = 64

// LoadManifest reads a reference manifest. It rejects the manifest as a
// whole when it is malformed (an unknown field, another schema_version, no
// entries) or when an entry cannot be named (a label that is not a short
// path segment, or one used twice). An entry with a missing licence, an
// unpinned SHA or another language loads, and Check refuses it.
func LoadManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read reference manifest: %w", err)
	}
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse reference manifest %s: %w", path, err)
	}
	if problem := manifestProblem(manifest); problem != "" {
		return Manifest{}, fmt.Errorf("reference manifest %s: %s", path, problem)
	}
	return manifest, nil
}

func manifestProblem(manifest Manifest) string {
	if manifest.SchemaVersion != "1" {
		return `schema_version must be "1"`
	}
	if len(manifest.References) == 0 {
		return "references is empty"
	}
	seen := map[string]bool{}
	for _, reference := range manifest.References {
		if !labelPattern.MatchString(reference.Label) {
			return fmt.Sprintf("label %q must be a lowercase path segment", reference.Label)
		}
		if len(reference.Label) > maxLabelLength {
			return fmt.Sprintf("label %q is longer than %d characters", reference.Label, maxLabelLength)
		}
		if seen[reference.Label] {
			return fmt.Sprintf("label %q appears twice", reference.Label)
		}
		seen[reference.Label] = true
	}
	return ""
}
