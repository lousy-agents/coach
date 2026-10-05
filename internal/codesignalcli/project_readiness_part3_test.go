package codesignalcli

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestAggregateReadinessKeepsProjectAndGlobalMiseChoicesDistinct proves an
// adapter rejection, a rejected mise_project choice, and a verified
// mise_global choice each surface as their own gap
// and next-action entry rather than colliding on a shared "resolve_package_manager"
// key, and prepare_compiler's surviving Choices names only the verified
// mise_global origin.
func TestAggregateReadinessKeepsProjectAndGlobalMiseChoicesDistinct(t *testing.T) {
	checks := ReadinessChecks{
		Compiler:       ReadinessCheck{State: ReadinessFail, Code: GapTypescriptCompilerMissing},
		PackageManager: ReadinessCheck{State: ReadinessFail, Code: GapPackageManagerVersionUnsupported, Kind: "yarn"},
	}
	miseChoices := []ReadinessMiseChoice{
		{Kind: "mise_project", Verified: false, Code: GapPackageManagerConfigUnverifiable},
		{Kind: "mise_global", Verified: true},
	}

	_, gaps, nextActions, _ := aggregateReadiness(checks, false, miseChoices)

	wantGaps := []ReadinessGap{
		{Code: GapTypescriptCompilerMissing},
		{Code: GapPackageManagerVersionUnsupported, PackageManagerKind: "yarn"},
		{Code: GapPackageManagerConfigUnverifiable, PackageManagerKind: "mise_project"},
	}
	if !reflect.DeepEqual(gaps, wantGaps) {
		t.Fatalf("gaps = %#v, want %#v", gaps, wantGaps)
	}

	var resolveKinds []string
	for _, action := range nextActions {
		if action.Kind == nextActionKindResolvePackageManager {
			resolveKinds = append(resolveKinds, action.PackageManagerKind)
		}
	}
	wantResolveKinds := []string{"yarn", "mise_project"}
	if !reflect.DeepEqual(resolveKinds, wantResolveKinds) {
		t.Fatalf("resolve_package_manager PackageManagerKind values = %#v, want %#v (yarn and mise_project must not collide into one entry)", resolveKinds, wantResolveKinds)
	}

	prepare, ok := findNextAction(nextActions, nextActionKindPrepareCompiler)
	if !ok {
		t.Fatalf("prepare_compiler missing from %#v, want it present with the verified mise_global choice", nextActions)
	}
	if !reflect.DeepEqual(prepare.Choices, []string{"mise_global"}) {
		t.Fatalf("prepare_compiler.Choices = %#v, want [mise_global] (project mise is rejected and distinct from global)", prepare.Choices)
	}
}

func testedNodeMajorFromMise(contents string) (int, error) {
	inTools := false
	for _, line := range strings.Split(contents, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[tools]" {
			inTools = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inTools = false
			continue
		}
		if !inTools {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok || strings.TrimSpace(key) != "node" {
			continue
		}
		raw := strings.Trim(strings.TrimSpace(value), `"`)
		majorPart, _, _ := strings.Cut(raw, ".")
		return strconv.Atoi(majorPart)
	}
	return 0, strconv.ErrSyntax
}
