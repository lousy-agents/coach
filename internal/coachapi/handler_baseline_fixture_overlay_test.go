package coachapi_test

// The handler baseline scans a filesystem tree. These bytes are the mutation
// fixtures that tree must contain so the hidden-input oracles still see two
// deterministic signals. They are written into a temp directory by
// baselineFixtureRoot and are not part of the committed scan corpus.
const baselineMutatingUpdateGo = `package widget

type Config struct {
	Name string
}

func UpdateName(cfg *Config, name string) {
	cfg.Name = name
}
`

const baselineMutatingResetGo = `package widget

func ResetName(cfg *Config) {
	cfg.Name = ""
}
`
