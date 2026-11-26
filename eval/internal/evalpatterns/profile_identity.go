package evalpatterns

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
