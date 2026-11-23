package projectconfig

// DiscoverOnDisk is the policy the working tree selects when none is named: what SelectOnDisk finds, unless the
// committed selection says the canonical path lies inside a submodule checkout. A file there cannot be committed
// here, so it is refused as the committed selection refuses it, not read as a policy.
func DiscoverOnDisk(dir string, committed Selection) (DiskSelection, error) {
	if err := committed.SubmoduleRefusal(); err != nil {
		return DiskSelection{}, err
	}
	return SelectOnDisk(dir, "")
}

// Deprecated reports whether the selected policy is the root project.json, the spelling that is being retired.
func (s DiskSelection) Deprecated() bool { return s.Path == DefaultPath }
