package evalharness

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ValidationVersion changes with any change inside an existing validation step: a new check in a step, or a new
// threshold. A new step changes the fingerprint on its own, through the step list. Bump it whenever either happens,
// so that a cached program is revalidated.
const ValidationVersion = "3"

// ValidationFingerprint is a digest of the ordered validation step IDs and the validation version.
func ValidationFingerprint(steps []string, version string) string {
	sum := sha256.Sum256([]byte(strings.Join(steps, "\x00") + "\x01" + version))
	return hex.EncodeToString(sum[:])
}
