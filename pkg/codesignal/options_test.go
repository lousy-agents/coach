package codesignal

import (
	"testing"
)

func TestNew_DoesNotAliasOrMutateOptions(t *testing.T) {
	opts := Options{IncludeResolved: true}

	b, err := New(opts)
	if err != nil {
		t.Fatalf("New(%+v) returned unexpected error: %v", opts, err)
	}
	if b == nil {
		t.Fatalf("New(%+v) returned nil Builder", opts)
	}

	opts.IncludeResolved = false
	if !b.options.IncludeResolved {
		t.Errorf("Builder.options must be a copy of the passed Options, not an alias; mutating caller's Options after New changed b.options")
	}
}
