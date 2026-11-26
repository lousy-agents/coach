package evalharness

import (
	"fmt"
	"os"
	"strings"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
	"github.com/lousy-agents/coach/eval/internal/evalpatterns"
)

// profileRequest is the part of a `coach-eval generate` command line that
// asks for cells drawn from a pattern profile (#872 Task 4).
type profileRequest struct {
	path  string
	count int
}

// profileCells draws the run's cells from the profile at path, after
// refusing what a profile run cannot be: a held-out run, which stays
// profile-free as preregistered, or one that also names its cells.
func profileCells(asked profileRequest, tuning bool, index int, dimensions evalgen.Cell, request GenerateRequest) ([]evalgen.Cell, string, error) {
	if err := profileConflicts(asked, tuning, index, dimensions, request.Panel); err != nil {
		return nil, "", err
	}
	raw, err := os.ReadFile(asked.path)
	if err != nil {
		return nil, "", fmt.Errorf("--profile: %w", err)
	}
	profile, err := evalgen.ReadProfile(raw, validSealedProfile)
	if err != nil {
		return nil, "", fmt.Errorf("--profile %s: %w", asked.path, err)
	}
	cells, err := evalgen.SampleCells(profile, asked.count, request.Seed)
	return cells, profile.Identity, err
}

func profileConflicts(asked profileRequest, tuning bool, index int, dimensions evalgen.Cell, panel string) error {
	given := givenDimensions(dimensions)
	switch {
	case panel == PanelHeldOut:
		return fmt.Errorf("--profile is refused for the %s panel, which stays profile-free, as preregistered", PanelHeldOut)
	case index != unset:
		return fmt.Errorf("--profile conflicts with --cell: the profile chooses the cells")
	case tuning:
		return fmt.Errorf("--profile conflicts with --tuning: the tuning cells are fixed")
	case len(given) > 0:
		return fmt.Errorf("--profile conflicts with %s: the profile chooses the cells", strings.Join(given, ", "))
	case asked.count == unset:
		return fmt.Errorf("--profile needs --count N, the number of cells to draw")
	}
	return nil
}

// validSealedProfile is a profile the committed schema accepts and whose
// identity is the one its contents seal to, so the identity a manifest
// records names the contents that chose its cell (AC-3.4).
func validSealedProfile(raw []byte) error {
	if err := evalpatterns.ValidateProfile(raw); err != nil {
		return err
	}
	return evalpatterns.VerifyProfileIdentity(raw)
}
