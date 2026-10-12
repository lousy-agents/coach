package main

import (
	"flag"
	"strconv"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestSeverityFloorWordingFollowsTheAcceptedFloors(t *testing.T) {
	floors := codesignal.SeverityFloors()
	names := make([]string, len(floors))
	quoted := make([]string, len(floors))
	for i, floor := range floors {
		names[i] = string(floor)
		quoted[i] = strconv.Quote(string(floor))
	}
	last := len(names) - 1

	flags := flag.NewFlagSet("codesignal", flag.ContinueOnError)
	registerCodesignalFlags(flags)

	var errorWant string
	for _, check := range flagValueChecks(codesignalFlags{minSeverity: "urgent", minSeveritySet: true}) {
		if check.flag == "min-severity" {
			errorWant = check.want
		}
	}

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"usage line", codesignalUsage, "[--min-severity " + strings.Join(names, "|") + "]"},
		{"flag help", flags.Lookup("min-severity").Usage, "(" + strings.Join(names[:last], ", ") + ", or " + names[last] + ")"},
		{"error text", errorWant, strings.Join(quoted[:last], ", ") + ", or " + quoted[last]},
	}
	for _, tc := range cases {
		if !strings.Contains(tc.got, tc.want) {
			t.Errorf("%s = %q, want it to list the accepted floors as %q", tc.name, tc.got, tc.want)
		}
	}
	if errorWant == "" {
		t.Fatal("no min-severity value check found")
	}
}
