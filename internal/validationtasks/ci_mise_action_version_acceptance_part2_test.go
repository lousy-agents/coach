package validationtasks

func miseTomlMinVersion(toml string) string {
	m := miseTomlMinVersionPattern.FindStringSubmatch(toml)
	if m == nil {
		return ""
	}
	return m[1]
}
