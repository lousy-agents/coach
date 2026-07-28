package codesignal

import "testing"

// Test_classifyStateDomain locks the full epic classification table
// (pkg/codesignal/rule_react_orchestration.go's reactStateDomainRules),
// including the order-sensitive navigation-before-selection precedence and
// the anchored regexes' "other" fallback traps, against every binding name
// the epic specifies.
func Test_classifyStateDomain(t *testing.T) {
	cases := []struct {
		binding string
		want    string
	}{
		{"activeView", "navigation"},
		{"selectedId", "selection"},
		{"filterText", "filtering"},
		{"hoverRow", "hover_focus"},
		{"pageIndex", "pagination"},
		{"sortKey", "sorting"},
		{"isModalOpen", "modal"},
		{"isLoading", "loading_error"},
		{"workspaceDraft", "other"},
		{"selectedView", "navigation"},
		{"faq", "other"},
		{"homepage", "other"},
		{"overview", "other"},
		{"showBanner", "other"},
		{"statusCode", "other"},
	}

	for _, tc := range cases {
		t.Run(tc.binding, func(t *testing.T) {
			got := classifyStateDomain(tc.binding)
			if got != tc.want {
				t.Errorf("classifyStateDomain(%q) = %q, want %q", tc.binding, got, tc.want)
			}
		})
	}
}
