package codesignalcli

import (
	"testing"
)

func body_projectTsCompilerMiseRowTest_56(t *testing.T, tc struct {
	name     string
	toml     string
	wantYear string
	wantOK   bool
}) {
	m := miseTomlMinVersionPattern.FindSubmatch([]byte(tc.toml))
	var year string
	var ok bool
	if m != nil {
		year, ok = miseCalverYear(string(m[1]))
	}
	if ok != tc.wantOK || year != tc.wantYear {
		t.Errorf("extraction of %q = (%q, %v), want (%q, %v)", tc.toml, year, ok, tc.wantYear, tc.wantOK)
	}
}

func body_projectTsCompilerMiseRowTest_86(t *testing.T, tc struct {
	name    string
	version string
	want    bool
}) {
	if got := isMiseToolVersionInRow(tc.version); got != tc.want {
		t.Errorf("isMiseToolVersionInRow(%q) = %v, want %v", tc.version, got, tc.want)
	}
}
