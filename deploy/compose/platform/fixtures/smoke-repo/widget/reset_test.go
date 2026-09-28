package widget

// ResetName clears cfg.Name through the pointer parameter.
// This fixture exists so a scan of these bytes emits mutates_input.
func ResetName(cfg *Config) {
	cfg.Name = ""
}
