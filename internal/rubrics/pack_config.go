package rubrics

// PackConfig controls deterministic judgment packing for local-LLM hidden-mutation
// batches. Zero fields are filled by ApplyPackConfigDefaults.
type PackConfig struct {
	MaxFindingsPerJudgmentPack      int
	MaxJudgmentPromptTokens         int
	JudgmentFileAffinityMinFindings int
	EvidenceWindowLines             int
}

// Default pack knobs (local-LLM oriented; see coach-api-platform-local-llm-judgment spec).
const (
	DefaultMaxFindingsPerJudgmentPack      = 4
	DefaultMaxJudgmentPromptTokens         = 3500
	DefaultJudgmentFileAffinityMinFindings = 5
	DefaultEvidenceWindowLines             = 15
)

// ApplyPackConfigDefaults returns cfg with zero-valued fields set to binding defaults.
func ApplyPackConfigDefaults(cfg PackConfig) PackConfig {
	if cfg.MaxFindingsPerJudgmentPack == 0 {
		cfg.MaxFindingsPerJudgmentPack = DefaultMaxFindingsPerJudgmentPack
	}
	if cfg.MaxJudgmentPromptTokens == 0 {
		cfg.MaxJudgmentPromptTokens = DefaultMaxJudgmentPromptTokens
	}
	if cfg.JudgmentFileAffinityMinFindings == 0 {
		cfg.JudgmentFileAffinityMinFindings = DefaultJudgmentFileAffinityMinFindings
	}
	if cfg.EvidenceWindowLines == 0 {
		cfg.EvidenceWindowLines = DefaultEvidenceWindowLines
	}
	return cfg
}
