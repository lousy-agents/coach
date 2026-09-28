package widget

// UpdateName returns a config with Name set to name.
func UpdateName(cfg Config, name string) Config {
	cfg.Name = name
	return cfg
}
