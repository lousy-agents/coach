package evalgen

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// The profile versions whose dimensions the weights are written against. A
// profile of another version may measure a dimension differently, so it is
// refused rather than read.
const (
	supportedProfileSchema    = "1"
	supportedProfileExtractor = "0.2.0"
)

// ProfileValidator checks a profile against the committed profile schema.
// The generation ring may not import the real-code ring that writes
// profiles, so the caller passes it (#872 AC-3.6).
type ProfileValidator func(raw []byte) error

// Profile is what the generator reads of a validated pattern profile: its
// identity, and for each cell dimension a profile can observe, how often it
// saw each in-range value.
type Profile struct {
	Identity string
	layers   map[int]int
	packages map[int]int
	entry    map[string]int
	depth    map[int]int
}

type profileDocument struct {
	SchemaVersion    string `json:"schema_version"`
	Identity         string `json:"identity"`
	ExtractorVersion string `json:"extractor_version"`
	Roles            map[string]struct {
		Packages map[string]int `json:"packages"`
	} `json:"roles"`
	Dimensions struct {
		Layers map[string]int `json:"layers"`
		Entry  map[string]int `json:"entry"`
		Depth  map[string]int `json:"depth"`
	} `json:"dimensions"`
}

// ReadProfile validates raw and reads the dimensions the generator weights
// cells by. A value outside the generator's range is left out, as the
// profile already lists it as not covered.
func ReadProfile(raw []byte, validate ProfileValidator) (Profile, error) {
	if err := validate(raw); err != nil {
		return Profile{}, fmt.Errorf("the profile does not validate: %w", err)
	}
	var document profileDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return Profile{}, fmt.Errorf("the profile cannot be read: %w", err)
	}
	switch {
	case document.SchemaVersion != supportedProfileSchema:
		return Profile{}, fmt.Errorf("the profile's schema_version %q is not %q", document.SchemaVersion, supportedProfileSchema)
	case document.ExtractorVersion != supportedProfileExtractor:
		return Profile{}, fmt.Errorf("the profile's extractor_version %q is not %q, which the weights are written against", document.ExtractorVersion, supportedProfileExtractor)
	}
	packages := map[string]int{}
	for _, role := range document.Roles {
		for value, count := range role.Packages {
			packages[value] += count
		}
	}
	return Profile{
		Identity: document.Identity,
		layers:   observedInRange(document.Dimensions.Layers, 3, 5),
		packages: observedInRange(packages, 2, 4),
		entry:    document.Dimensions.Entry,
		depth:    observedInRange(document.Dimensions.Depth, 2, 5),
	}, nil
}

func observedInRange(counts map[string]int, low, high int) map[int]int {
	kept := map[int]int{}
	for bucket, count := range counts {
		if value, err := strconv.Atoi(bucket); err == nil && value >= low && value <= high {
			kept[value] += count
		}
	}
	return kept
}
