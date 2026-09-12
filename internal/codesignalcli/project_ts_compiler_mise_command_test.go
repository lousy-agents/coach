package codesignalcli

import (
	"context"
	"testing"
)

func TestMiseInstallToolSpec(t *testing.T) {
	got := miseInstallToolSpec("7.0.2")
	want := "npm:typescript@7.0.2"
	if got != want {
		t.Errorf("miseInstallToolSpec(%q) = %q, want %q", "7.0.2", got, want)
	}
}

func TestInstallMiseTypescriptRefusesWhenUntrusted(t *testing.T) {
	result := installMiseTypescript(context.Background(), compilerOriginMiseProject, "7.0.2", miseSetupTrust{code: GapPackageManagerConfigUnverifiable})
	if result.Trusted {
		t.Fatalf("installMiseTypescript() = %+v, want Trusted=false", result)
	}
	if result.Attempted {
		t.Errorf("installMiseTypescript() = %+v, an untrusted scope must never attempt mise install", result)
	}
	if result.Code != GapPackageManagerConfigUnverifiable {
		t.Errorf("installMiseTypescript() Code = %q, want %q", result.Code, GapPackageManagerConfigUnverifiable)
	}
}

func TestRunMiseInstallInsulatedReportsNotAttemptedWhenMiseAbsent(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	attempted, observed, insulationFailed, _ := runMiseInstallInsulated(context.Background(), "npm:typescript@7.0.2")
	if attempted {
		t.Errorf("runMiseInstallInsulated() attempted = true with mise absent from PATH, want false")
	}
	if observed {
		t.Errorf("runMiseInstallInsulated() observed = true with mise absent from PATH, want false")
	}
	if insulationFailed {
		t.Errorf("runMiseInstallInsulated() insulationFailed = true with mise absent from PATH, want false -- mise being absent is a different failure than insulation itself failing")
	}
}
