package projectconfig

import "testing"

func TestScanErrorMarksOnlyAFileThatIsUnusableForWhatItHolds(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   bool
	}{
		{"not a valid policy", refusalInvalid, true},
		{"over the size limit", refusalOversize, true},
		{"a symbolic link", refusalLink, false},
		{"not a regular file", refusalNotRegular, false},
		{"unreadable", refusalUnreadable, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectUnusableOnDisk(t, tt.reason, tt.want) })
	}
}

func expectUnusableOnDisk(t *testing.T, reason string, want bool) {
	t.Helper()
	refusal := &DiskPolicyRefusal{Path: "p.json", Reason: reason}
	if got := configErrorOf(t, refusal.ScanError(false, false, nil)).UnusableOnDisk; got != want {
		t.Fatalf("UnusableOnDisk = %v, want %v", got, want)
	}
}
