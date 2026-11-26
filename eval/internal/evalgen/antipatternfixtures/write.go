package antipatternfixtures

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// expectation is #871's expect.json, field for field.
type expectation struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Seed   string `json:"seed"`
	Status string `json:"status"`
	Expect struct {
		Bad  any `json:"bad"`
		Good any `json:"good"`
	} `json:"expect"`
	Note string `json:"note"`
}

// sourceSuffix keeps the fixture's Go files out of the repository's own
// scan and build, as #871's hand-authored fixtures do.
const sourceSuffix = ".txt"

func writeFixture(out string, template Template, seen observation, identity string, bad, good map[string]string) error {
	dir := filepath.Join(out, template.ID)
	if err := writeFiles(filepath.Join(dir, "bad"), bad, sourceSuffix); err != nil {
		return err
	}
	if err := writeFiles(filepath.Join(dir, "good"), good, sourceSuffix); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(expectationOf(template, seen, identity), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "expect.json"), append(raw, '\n'), 0o644)
}

func expectationOf(template Template, seen observation, identity string) expectation {
	written := expectation{
		ID: template.ID, Name: template.Name, Status: template.Status,
		Source: "#871 catalogue entry " + template.ID + " (eval/fixtures/antipatterns/" + template.ID + "), scaled by a pattern profile",
		Seed:   "profile:" + identity,
		Note:   seen.note(identity),
	}
	written.Expect.Bad, written.Expect.Good = "none", "none"
	if template.Expect != nil {
		written.Expect.Bad = template.Expect(seen.value)
	}
	return written
}
