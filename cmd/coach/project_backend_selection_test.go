package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

func TestSelectProjectBackendSaysWhyNoBackendRuns(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name            string
		resolveErr      error
		language        string
		wantBackend     bool
		wantUnavailable string
		wantErr         error
	}{
		{"a backend for the language", nil, "go", true, "", nil},
		{"the registry says none is available", &codesignalcli.ProjectBackendUnavailableError{Message: "not here"}, "go", false, "not here", nil},
		{"a failure that is not unavailability", boom, "go", false, "", boom},
		{"a registered language with no backend yet", nil, "cobol", false, `no project-analysis backend is available for language "cobol"`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertBackendSelection(t, c.resolveErr, c.language, c.wantBackend, c.wantUnavailable, c.wantErr)
		})
	}
}

func assertBackendSelection(t *testing.T, resolveErr error, language string, wantBackend bool, wantUnavailable string, wantErr error) {
	t.Helper()
	ports := defaultScanPorts()
	ports.resolveBackend = func(string) error { return resolveErr }

	backend, unavailable, err := selectProjectBackend(ports, language)

	if (backend != nil) != wantBackend {
		t.Errorf("backend present = %v, want %v", backend != nil, wantBackend)
	}
	if !strings.Contains(unavailable, wantUnavailable) || (wantUnavailable == "" && unavailable != "") {
		t.Errorf("unavailable = %q, want it to contain %q", unavailable, wantUnavailable)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}
