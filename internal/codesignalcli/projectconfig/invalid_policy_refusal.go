package projectconfig

import "errors"

// invalidPolicyRefusal is the refusal of a policy that was read and is not valid, with the one detail it can name
// without quoting the policy.
func invalidPolicyRefusal(path string, invalid *invalidDiskPolicyError) *DiskPolicyRefusal {
	refusal := &DiskPolicyRefusal{Path: path, Reason: refusalInvalid}
	var language *InvalidLanguageError
	if errors.As(invalid, &language) {
		refusal.Detail = language.Error()
	}
	return refusal
}
