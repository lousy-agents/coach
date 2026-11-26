package main_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/gomega"
)

func copyDirectory(from, to string) {
	ExpectWithOffset(1, filepath.WalkDir(from, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, content, 0o644)
	})).To(Succeed())
}

// coveredFixtureIDs are the fixtures a table reports as covered, each of which must pass on both variants.
func coveredFixtureIDs(table antipatternTable) map[string]bool {
	covered := map[string]bool{}
	for _, fixture := range table.Fixtures {
		if fixture.Status == "covered" {
			covered[fixture.ID] = true
			ExpectWithOffset(1, fixture.Bad.Result).To(Equal("pass"), fixture.ID)
			ExpectWithOffset(1, fixture.Good.Result).To(Equal("pass"), fixture.ID)
		}
	}
	return covered
}

// expectKnownMissesSilent requires every not_covered fixture to be one Coach is still silent on, with a reason to promote it if not.
func expectKnownMissesSilent(table antipatternTable) {
	for _, fixture := range table.Fixtures {
		if fixture.Status == "not_covered" {
			wantBad := "miss"
			if fixture.ID == "bypass-without-required-layer" {
				wantBad = "incomplete"
			}
			ExpectWithOffset(1, fixture.Bad.Result).To(Equal(wantBad), fixture.ID+": Coach is silent on its bad variant today; if it is not, promote the fixture")
			ExpectWithOffset(1, fixture.Good.Result).To(Equal("miss"), fixture.ID)
		}
	}
}
