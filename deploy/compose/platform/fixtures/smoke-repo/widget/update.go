package widget

type Config struct {
	Name string
}

// Pointer receiver mutation — fixture input for deterministic mutates_input.
func UpdateName(cfg *Config, name string) {
	cfg.Name = name
}
