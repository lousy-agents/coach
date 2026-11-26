package evalpatterns

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// extractEntry checks out the entry from clone into a temporary directory,
// which it removes, extracts its profile and writes it under out. It
// returns the profile's identity.
func extractEntry(reference Reference, clone, out string) (string, error) {
	if clone == "" {
		return "", reference.refuse("no --clone was given for it")
	}
	work, err := os.MkdirTemp("", "coach-eval-patterns-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	checkout := filepath.Join(work, "checkout")
	if err := CheckoutReference(reference, clone, checkout); err != nil {
		return "", err
	}
	raw, err := ExtractProfile(reference, checkout)
	if err != nil {
		return "", err
	}
	return writeProfile(raw, reference.Label, out)
}

func writeProfile(raw []byte, label, out string) (string, error) {
	var sealed struct {
		Identity string `json:"identity"`
	}
	if err := json.Unmarshal(raw, &sealed); err != nil {
		return "", err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(out, label+".profile.json"), raw, 0o644); err != nil {
		return "", err
	}
	return sealed.Identity, nil
}
