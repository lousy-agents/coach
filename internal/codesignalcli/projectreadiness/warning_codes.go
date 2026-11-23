package projectreadiness

// The policy warnings say how the policy the check read from disk relates to the one the analyzed revision holds. None
// of them changes the status.
const (
	// WarnProjectConfigDeprecated is a policy read from the root project.json instead of the default policy file.
	WarnProjectConfigDeprecated = "project_config_deprecated"
	// WarnPolicyNotCommitted is a policy on disk that the analyzed revision holds nothing for at its path.
	WarnPolicyNotCommitted = "project_policy_not_committed"
	// WarnPolicyDiffersFromHead is a policy on disk that differs from the analyzed revision's copy, or that the
	// revision holds as a link or the index holds as a conflict.
	WarnPolicyDiffersFromHead = "project_policy_differs_from_head"
	// WarnPolicyHeadUnreadable is a policy on disk whose copy at the analyzed revision could not be read, so nothing
	// can be said about how the two relate.
	WarnPolicyHeadUnreadable = "project_policy_head_unreadable"
)
