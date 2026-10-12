package semantics

import (
	"context"
	"errors"
	"testing"
)

// AC-1.8: if the supplied context is already cancelled, validate must report
// (nil, ctx.Err()) before doing any parsing work.
func TestValidate_RejectsAlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := validate(ctx, []byte("package main\n"), LanguageGo, 0, nil)

	if result != nil {
		t.Errorf("AC-1.8: validate with a cancelled context: got result %+v, want nil", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("AC-1.8: validate with a cancelled context: got err %v, want errors.Is(err, context.Canceled)", err)
	}
}

// AC-1.4: empty content must be rejected with an error matching
// ErrEmptyContent.
func TestValidate_RejectsEmptyContent(t *testing.T) {
	result, err := validate(context.Background(), []byte{}, LanguageGo, 0, nil)

	if result != nil {
		t.Errorf("AC-1.4: validate with empty content: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrEmptyContent) {
		t.Errorf("AC-1.4: validate with empty content: got err %v, want errors.Is(err, ErrEmptyContent)", err)
	}
}

// AC-1.6: content exceeding the configured max size must be rejected with an
// error matching ErrFileTooLarge. Uses a small explicit max (10 bytes)
// rather than the real 2 MiB default so the test fixture stays tiny.
func TestValidate_RejectsContentOverMaxFileBytes(t *testing.T) {
	const maxFileBytes = 10
	content := []byte("this is more than ten bytes")

	result, err := validate(context.Background(), content, LanguageGo, maxFileBytes, nil)

	if result != nil {
		t.Errorf("AC-1.6: validate with content over max size: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("AC-1.6: validate with %d-byte content and max %d: got err %v, want errors.Is(err, ErrFileTooLarge)", len(content), maxFileBytes, err)
	}
}

// AC-1.7: content containing a NUL byte must be rejected with an error
// matching ErrBinaryContent.
func TestValidate_RejectsContentContainingNULByte(t *testing.T) {
	content := []byte("package main\x00\n")

	result, err := validate(context.Background(), content, LanguageGo, 0, nil)

	if result != nil {
		t.Errorf("AC-1.7: validate with a NUL byte in content: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrBinaryContent) {
		t.Errorf("AC-1.7: validate with a NUL byte in content: got err %v, want errors.Is(err, ErrBinaryContent)", err)
	}
}
