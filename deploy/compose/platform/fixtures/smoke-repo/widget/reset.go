package widget

// Pointer receiver mutation — fixture input for deterministic mutates_input.
func ResetName(cfg *Config) {
	cfg.Name = ""
}
