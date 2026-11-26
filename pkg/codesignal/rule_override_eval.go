//go:build coach_eval_override

package codesignal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	evalOverrideVariable = "COACH_EVAL_OVERRIDE"
	evalOverrideBanner   = "coach: eval override active: "
)

type evalOverride struct {
	thresholds map[string]int
	disabled   map[string]bool
}

// Loaded while the package initialises so an invalid override stops the
// process before any file is analysed.
var activeOverride = loadEvalOverride()

func loadEvalOverride() evalOverride {
	raw := os.Getenv(evalOverrideVariable)
	if raw == "" {
		return evalOverride{}
	}

	parsed, err := parseEvalOverride(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "coach: invalid %s: %v\n", evalOverrideVariable, err)
		os.Exit(2)
	}

	var compact bytes.Buffer
	if json.Compact(&compact, []byte(raw)) != nil {
		compact.WriteString(raw)
	}
	fmt.Fprintln(os.Stderr, evalOverrideBanner+compact.String())
	return parsed
}

func parseEvalOverride(raw string) (evalOverride, error) {
	var document struct {
		Thresholds map[string]json.RawMessage `json:"thresholds"`
		Disable    []string                   `json:"disable"`
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return evalOverride{}, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return evalOverride{}, errors.New("unexpected content after the JSON object")
	}

	thresholds, err := validatedThresholds(document.Thresholds)
	if err != nil {
		return evalOverride{}, err
	}
	disabled, err := validatedDisabledRules(document.Disable)
	if err != nil {
		return evalOverride{}, err
	}
	return evalOverride{thresholds: thresholds, disabled: disabled}, nil
}

func ruleThreshold(ruleID string, compiled int) int {
	if value, overridden := activeOverride.thresholds[ruleID]; overridden {
		return value
	}
	return compiled
}

func withoutDisabledRules(signals []Signal) []Signal {
	if len(activeOverride.disabled) == 0 {
		return signals
	}
	kept := make([]Signal, 0, len(signals))
	for _, signal := range signals {
		if !activeOverride.disabled[signal.RuleID] {
			kept = append(kept, signal)
		}
	}
	return kept
}
