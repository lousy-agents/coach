package main

import (
	"strings"
	"testing"
)

func TestValidateSuggestProjectConfigFlags(t *testing.T) {
	t.Run("rejects an unenumerated flag by name", func(t *testing.T) {
		f := codesignalFlags{baseline: true, suggestProjectConfig: true}
		setFlags := map[string]bool{
			"suggest-project-config": true,
			"baseline":               true,
			"totally-new-flag":       true,
		}

		got := validateSuggestProjectConfigFlags(f, setFlags, nil, 1, 0)

		if got == "" {
			t.Fatalf("validateSuggestProjectConfigFlags returned no error for an unenumerated flag; expected rejection")
		}
		if !strings.Contains(got, "totally-new-flag") {
			t.Errorf("validateSuggestProjectConfigFlags = %q, want a message naming the unenumerated flag", got)
		}
	})

	t.Run("allows project-language when its value is typescript", func(t *testing.T) {
		f := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript"}
		setFlags := map[string]bool{
			"suggest-project-config": true,
			"baseline":               true,
			"project-language":       true,
		}

		got := validateSuggestProjectConfigFlags(f, setFlags, nil, 1, 0)

		if got != "" {
			t.Fatalf("validateSuggestProjectConfigFlags = %q, want no error for --project-language typescript", got)
		}
	})

	t.Run("rejects project-language when its value is go", func(t *testing.T) {
		f := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "go"}
		setFlags := map[string]bool{
			"suggest-project-config": true,
			"baseline":               true,
			"project-language":       true,
		}

		got := validateSuggestProjectConfigFlags(f, setFlags, nil, 1, 0)

		if got == "" {
			t.Fatalf("validateSuggestProjectConfigFlags returned no error for --project-language go; expected rejection")
		}
		if !strings.Contains(got, "project-language") {
			t.Errorf("validateSuggestProjectConfigFlags = %q, want a message naming project-language", got)
		}
	})
}
