package main

// The explicit-policy specs that run on every platform share these two declarations with the specs that
// need a Unix-only fixture (a named pipe), which are built on Unix alone.

const explicitPolicyPath = "policies/p.json"

// commitExplicitPolicyRepo commits a valid explicit policy plus valid
// discovered policies, so a read from HEAD or a fallback to a discovered policy
// would produce a report that these specs could see.
func commitExplicitPolicyRepo() string {
	repo := commitScannableGoRepo()
	commitFile(repo, explicitPolicyPath, singleRootPolicyJSON)
	commitFile(repo, coachConfigPath, singleRootPolicyJSON)
	commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
	return repo
}
