package render

import (
	"reflect"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A repository plants the strings --check-project prints: the version a package.json declares, the
// versions and detail text read from what is installed, the roots a policy names, the name of any
// uncommitted file. hostileValue is what a clone can put in any of them: an OSC title change, a CSI
// screen clear, a BEL, and an 8-bit CSI that a UTF-8 terminal reads as the same thing.
const hostileValue = "7.0.2\x1b]0;PWNED\x07\x1b[2J\u009b"

// everyStringSetTo walks a value and sets every string it holds, in every nested struct and slice, so a
// field the renderer prints cannot be missed by a list kept by hand, now or when one is added.
func everyStringSetTo(v reflect.Value, text string) {
	switch v.Kind() {
	case reflect.String:
		if v.CanSet() {
			v.SetString(text)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			everyStringSetTo(v.Field(i), text)
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.String || v.Type().Elem().Kind() == reflect.Struct {
			v.Set(reflect.MakeSlice(v.Type(), 1, 1))
			everyStringSetTo(v.Index(0), text)
		}
	}
}

// resultThatPrintsEveryField is a Result in which every string is hostileValue, with the few switches the
// renderer reads set so that each field it can print is printed: a failing check prints its declared
// version, a mismatch warning prints its fields, and relevant changes print the dirty paths.
func resultThatPrintsEveryField() projectreadiness.Result {
	var result projectreadiness.Result
	everyStringSetTo(reflect.ValueOf(&result).Elem(), hostileValue)
	for _, check := range []*projectreadiness.Check{&result.Checks.ProjectShape, &result.Checks.Policy, &result.Checks.Node, &result.Checks.Runtime, &result.Checks.Compiler, &result.Checks.PackageManager} {
		check.State = projectreadiness.Fail
	}
	result.Warnings[0].Code = projectreadiness.WarnCompilerDeclarationMismatch
	result.DirtyWorktree.RelevantChanges = true
	return result
}
