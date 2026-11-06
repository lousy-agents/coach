package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestAuthorableMissingPolicy(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"a policy that does not exist", &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Remedy: projectreadiness.RemedyAuthor}, true},
		{"the same, wrapped", fmt.Errorf("wrapped: %w", &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Remedy: projectreadiness.RemedyAuthor}), true},
		{"a missing policy whose remedy nobody set", &projectconfig.ConfigError{Kind: projectconfig.KindMissing}, false},
		{"a policy committed at the revision", &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Remedy: projectreadiness.RemedyRestore}, false},
		{"a path that cannot be created", &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Remedy: projectreadiness.RemedyNone}, false},
		{"an invalid policy", &projectconfig.ConfigError{Kind: projectconfig.KindInvalid}, false},
		{"not a config error", errors.New("boom"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertAuthorable(t, c.err, c.want)
		})
	}
}

func assertAuthorable(t *testing.T, err error, want bool) {
	t.Helper()
	if got := authorableMissingPolicy(err); got != want {
		t.Errorf("authorableMissingPolicy = %v, want %v", got, want)
	}
}
