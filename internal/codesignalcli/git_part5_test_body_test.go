package codesignalcli

import (
	"testing"
)

func body_gitPart5Test_93(t *testing.T, tt struct {
	name    string
	payload []byte
	want    []nameStatusRecord
	wantErr bool
}) {
	got, err := parseNameStatusZ(tt.payload)
	if tt.wantErr {
		if err == nil {
			t.Fatalf("parseNameStatusZ(%q): want error, got nil", tt.payload)
		}
		return
	}
	if err != nil {
		t.Fatalf("parseNameStatusZ(%q): unexpected error: %v", tt.payload, err)
	}
	if !recordsEqual(got, tt.want) {
		t.Errorf("parseNameStatusZ(%q) = %#v, want %#v", tt.payload, got, tt.want)
	}
}
