package evalharness

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
)

// coachBinary is the coach whose report on a base is recorded: COACH_BIN, else coach on PATH. The hash of its
// content joins the cache key, because a program's record is only as good as the binary that made it.
type coachBinary struct {
	path string
	hash string
}

func findCoachBinary() coachBinary {
	path := os.Getenv("COACH_BIN")
	if path == "" {
		found, err := exec.LookPath("coach")
		if err != nil {
			return coachBinary{}
		}
		path = found
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return coachBinary{}
	}
	sum := sha256.Sum256(content)
	return coachBinary{path: path, hash: hex.EncodeToString(sum[:])}
}

func (c coachBinary) found() bool { return c.path != "" }
