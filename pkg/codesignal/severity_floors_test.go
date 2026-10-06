package codesignal

import "testing"

func TestSeverityFloorsListsEveryRankedSeverityHighestFirst(t *testing.T) {
	floors := SeverityFloors()

	if len(floors) != len(severityOrder) {
		t.Fatalf("SeverityFloors() = %q, want one entry per severity in %q", floors, severityOrder)
	}
	for i, floor := range floors {
		if want := severityOrder[len(severityOrder)-1-i]; floor != want {
			t.Fatalf("SeverityFloors()[%d] = %q, want %q (highest first)", i, floor, want)
		}
	}
}

func TestSeverityFloorsReturnsACopy(t *testing.T) {
	floors := SeverityFloors()
	floors[0] = "mutated"

	if got := SeverityFloors()[0]; got == "mutated" {
		t.Fatal("mutating the returned slice changed what SeverityFloors reports")
	}
	if got := severityOrder[len(severityOrder)-1]; got == "mutated" {
		t.Fatal("mutating the returned slice changed severityOrder")
	}
}
