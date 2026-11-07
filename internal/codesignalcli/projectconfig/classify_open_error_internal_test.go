package projectconfig

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestAnOpenThatFindsTheEntryGoneIsMissingNotUnreadable(t *testing.T) {
	err := classifyOpenError("p.json", &fs.PathError{Op: "open", Path: "p.json", Err: fs.ErrNotExist})

	var found *ConfigError
	if !errors.As(err, &found) || found.Kind != KindMissing {
		t.Errorf("classifyOpenError = %#v; want a missing-policy error, because the entry disappeared between the stat and the open", err)
	}
}

func TestAnOpenRefusedForAnyOtherReasonIsUnreadable(t *testing.T) {
	err := classifyOpenError("p.json", &fs.PathError{Op: "open", Path: "p.json", Err: fs.ErrPermission})

	var found *ConfigError
	if !errors.As(err, &found) || found.Kind != KindInvalid || !strings.Contains(found.Message, "cannot be read on disk") {
		t.Errorf("classifyOpenError = %#v; want an invalid-policy error that says the file cannot be read on disk", err)
	}
}
