package evalharness

import "github.com/lousy-agents/coach/eval/internal/evalgen"

// OracleGrades counts the tests each oracle reported as passed under native G1.
type OracleGrades struct {
	FirstOracleTests    int
	FollowOnOracleTests int
}

// GradeOracles runs G2 on each oracle of a Go program, then native G1: the tests the program lists and the hidden
// tests must each report pass. A finding or a test that did not pass rejects the program at its own step. Native G1 is
// go test's own report, not the nonce channel (#791), so nothing it records may be read as that G1.
func GradeOracles(program evalgen.Program) (OracleGrades, error) {
	trees := oracleTrees(program)
	for _, tree := range trees {
		if err := gradeG2(program, tree); err != nil {
			return OracleGrades{}, err
		}
	}
	var counts []int
	for _, tree := range trees {
		count, err := gradeNativeG1(program, tree)
		if err != nil {
			return OracleGrades{}, err
		}
		counts = append(counts, count)
	}
	return OracleGrades{FirstOracleTests: counts[0], FollowOnOracleTests: counts[1]}, nil
}
