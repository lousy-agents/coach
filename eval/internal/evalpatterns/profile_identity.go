package evalpatterns

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
)

func itoa(n int) string { return strconv.Itoa(n) }

// canonicalJSON is value as JSON with every object's keys sorted and no
// insignificant whitespace: decoding into generic maps and encoding again
// sorts the keys of structs as well as of maps.
func canonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return json.Marshal(generic)
}

// sealed is p in canonical JSON, with its identity set to the sha256 of
// the canonical JSON of p without one (AC-5.1), and a trailing newline.
func sealed(p profile) ([]byte, error) {
	p.Identity = ""
	unsealed, err := canonicalJSON(p)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(unsealed)
	p.Identity = hex.EncodeToString(sum[:])
	raw, err := canonicalJSON(p)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// VerifyProfileIdentity checks that raw's identity is the one its contents
// seal to, so a profile edited after it was written cannot carry the
// identity of the profile it was.
func VerifyProfileIdentity(raw []byte) error {
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return fmt.Errorf("profile is not JSON: %w", err)
	}
	carried, _ := document["identity"].(string)
	delete(document, "identity")
	unsealed, err := canonicalJSON(document)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(unsealed)
	if content := hex.EncodeToString(sum[:]); carried != content {
		return fmt.Errorf("its identity is %s, but its contents hash to %s", carried, content)
	}
	return nil
}

// ValidateSealedProfile accepts a profile that the committed schema accepts
// and whose identity is the one its contents seal to, so whatever records
// that identity names the contents it was derived from.
func ValidateSealedProfile(raw []byte) error {
	if err := ValidateProfile(raw); err != nil {
		return err
	}
	return VerifyProfileIdentity(raw)
}
