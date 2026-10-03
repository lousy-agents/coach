package widget

// ResetName returns a config whose Name is cleared.
func ResetName(cfg Config) Config {
	cfg.Name = ""
	return cfg
}
