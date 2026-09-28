package widget

// Config holds a name.
type Config struct {
	Name string
}

// UpdateName returns a config with Name set to name.
func UpdateName(cfg Config, name string) Config {
	cfg.Name = name
	return cfg
}
