package widget

// UpdateName mutates cfg through its pointer parameter instead of
// returning a new value. This fixture exists so a scan of these bytes
// emits mutates_input.
func UpdateName(cfg *Config, name string) {
	cfg.Name = name
}
