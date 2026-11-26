package evalgen_test

import (
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func validateWith(t *testing.T, node string, limit time.Duration) (error, time.Duration) {
	t.Helper()
	resolved := tools(t)
	resolved.Node, resolved.Limit = node, limit
	started := time.Now()
	err := evalgen.ValidateTypeScript(program(t), resolved)
	return err, time.Since(started)
}

func validateTSCWith(t *testing.T, tsc string, limit time.Duration) (error, time.Duration) {
	t.Helper()
	resolved := tools(t)
	resolved.TSC, resolved.Limit = tsc, limit
	started := time.Now()
	err := evalgen.ValidateTypeScript(program(t), resolved)
	return err, time.Since(started)
}

func TestAValidationRunThatOutlastsItsLimitIsKilledAndSaysSo(t *testing.T) {
	node := writeScript(t, t.TempDir(), "node", "sleep 60")

	err, took := validateWith(t, node, 2*time.Second)

	if err == nil || !strings.Contains(err.Error(), "time limit") {
		t.Fatalf("got %v, want an error that names the time limit", err)
	}
	if took > 30*time.Second {
		t.Fatalf("validation took %s: the limit did not stop the run", took)
	}
}

func TestAValidationRunTscThatOutlastsItsLimitIsKilledAndSaysSo(t *testing.T) {
	tsc := writeScript(t, t.TempDir(), "tsc", "sleep 60")

	err, took := validateTSCWith(t, tsc, 2*time.Second)

	if err == nil || !strings.Contains(err.Error(), "time limit") {
		t.Fatalf("got %v, want an error that names the time limit", err)
	}
	if took > 30*time.Second {
		t.Fatalf("validation took %s: the limit did not stop tsc", took)
	}
}

func TestAValidationRunWhoseLeftoverProcessHoldsItsOutputIsNotHeldOpen(t *testing.T) {
	node := writeScript(t, t.TempDir(), "node", "sleep 60 &\nexit 0")

	_, took := validateWith(t, node, 2*time.Second)

	if took > 30*time.Second {
		t.Fatalf("validation took %s: a background process held the run open", took)
	}
}

func TestAZeroLimitMeansTheDefaultNotNoTime(t *testing.T) {
	node := writeScript(t, t.TempDir(), "node", "sleep 1\nexit 0")

	err, _ := validateWith(t, node, 0)

	if err != nil && strings.Contains(err.Error(), "time limit") {
		t.Fatalf("a node that takes one second hit the limit: %v", err)
	}
}

func TestTheDefaultLimitIsOneMinute(t *testing.T) {
	if evalgen.DefaultValidationLimit != time.Minute {
		t.Fatalf("default limit is %s", evalgen.DefaultValidationLimit)
	}
}
