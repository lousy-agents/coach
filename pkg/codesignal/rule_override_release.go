//go:build !coach_eval_override

package codesignal

// The release build carries no override: both seams return their input, so
// the compiler inlines them away and nothing of the override reaches the binary.

func ruleThreshold(_ string, compiled int) int { return compiled }

func withoutDisabledRules(signals []Signal) []Signal { return signals }
