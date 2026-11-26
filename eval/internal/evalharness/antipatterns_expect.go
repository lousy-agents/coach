package evalharness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	statusCovered    = "covered"
	statusNotCovered = "not_covered"
)

// expectedFinding is one signal a scan of a variant must report: its rule, the file it is in and its subject.
type expectedFinding struct {
	RuleID  string `json:"rule_id"`
	Path    string `json:"path"`
	Subject string `json:"subject"`
}

// expectedFindings is a list of findings, or the string "none" for an empty one.
type expectedFindings []expectedFinding

func (f *expectedFindings) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte(`"none"`)) {
		*f = nil
		return nil
	}
	var list []expectedFinding
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf(`must be a list of {rule_id, path, subject} or "none": %v`, err)
	}
	*f = list
	return nil
}

// antipattern is one fixture's expect.json.
type antipattern struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Seed   string `json:"seed"`
	Status string `json:"status"`
	Expect struct {
		Bad  expectedFindings `json:"bad"`
		Good expectedFindings `json:"good"`
	} `json:"expect"`
	dir string
}

// loadAntipattern reads and checks the expect.json of the fixture in dir.
func loadAntipattern(dir string) (antipattern, error) {
	name := filepath.Base(dir)
	content, err := os.ReadFile(filepath.Join(dir, "expect.json"))
	if err != nil {
		return antipattern{}, fmt.Errorf("fixture %s: no expect.json: %v", name, err)
	}
	var fixture antipattern
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		return antipattern{}, fmt.Errorf("fixture %s: expect.json is not valid: %v", name, err)
	}
	if fixture.ID != name {
		return antipattern{}, fmt.Errorf("fixture %s: expect.json has id %q, which is not the directory's name", name, fixture.ID)
	}
	if fixture.Status != statusCovered && fixture.Status != statusNotCovered {
		return antipattern{}, fmt.Errorf("fixture %s: status %q must be %s or %s", name, fixture.Status, statusCovered, statusNotCovered)
	}
	if fixture.Status == statusCovered && len(fixture.Expect.Bad) == 0 {
		return antipattern{}, fmt.Errorf("fixture %s: a covered fixture must expect at least one finding on its bad variant", name)
	}
	fixture.dir = dir
	return fixture, nil
}
