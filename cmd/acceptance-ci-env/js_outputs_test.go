package main

import "testing"

func TestWantsTSProjectBackendReadsOnlyALabelFilterThatSelectsIt(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"the = form", []string{"./cmd/coach/...", "-args", "-ginkgo.label-filter=ts-project-backend"}, true},
		{"the two-argument form", []string{"-args", "-ginkgo.label-filter", "ts-project-backend"}, true},
		{"one term of several", []string{"-ginkgo.label-filter=ts-sidecar-integration || ts-project-backend"}, true},
		{"a negated label", []string{"-ginkgo.label-filter=!ts-project-backend"}, false},
		{"a negated label beside another", []string{"-ginkgo.label-filter=ts-sidecar-integration && !ts-project-backend"}, false},
		{"no label filter", []string{"./..."}, false},
		{"the label named outside a label filter", []string{"-ginkgo.focus=ts-project-backend"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { expectWants(t, tc.args, tc.want) })
	}
}

func expectWants(t *testing.T, args []string, want bool) {
	t.Helper()
	if got := wantsTSProjectBackend(args); got != want {
		t.Errorf("wantsTSProjectBackend(%q) = %v, want %v", args, got, want)
	}
}
