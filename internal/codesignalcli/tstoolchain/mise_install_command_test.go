package tstoolchain

import (
	"reflect"
	"testing"
)

func TestMiseInstallCommandIsExactlyTheFrozenTemplate(t *testing.T) {
	got := MiseInstallCommand("7.0.2")
	want := "mise install npm:typescript@7.0.2"
	if got != want {
		t.Errorf("miseInstallCommand(%q) = %q, want %q", "7.0.2", got, want)
	}
}

func TestMiseWhereCommandIsExactlyTheFrozenTemplate(t *testing.T) {
	if miseWhereCommand != "mise where" {
		t.Errorf("miseWhereCommand = %q, want %q", miseWhereCommand, "mise where")
	}
}

// TestMiseInstallCommandTakesExactlyOneVersionParameter pins that the row's
// install target names exactly one version, never a secondary or fallback
// version. It asserts on MiseInstallCommand's own reflected
// signature -- an oracle independent of the template literal pinned by
// TestMiseInstallCommandIsExactlyTheFrozenTemplate -- so a future signature
// change to accept a collection of versions (e.g. []string) fails this test
// directly rather than passing because both sides of a comparison moved
// together.
func TestMiseInstallCommandTakesExactlyOneVersionParameter(t *testing.T) {
	fnType := reflect.TypeOf(MiseInstallCommand)
	if fnType.NumIn() != 1 {
		t.Fatalf("miseInstallCommand has %d parameters, want exactly 1 (the single resolved version, never a secondary/fallback)", fnType.NumIn())
	}
	if got := fnType.In(0).Kind(); got != reflect.String {
		t.Errorf("miseInstallCommand's parameter kind = %v, want %v (a single version, not a slice/collection)", got, reflect.String)
	}
}
