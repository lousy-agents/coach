package domain

import "testing"

func TestAnyLayerPrefixContainsUsesTheSingleDefinitionOfLayerMembership(t *testing.T) {
	tests := []struct {
		name     string
		prefixes []string
		key      string
		want     bool
	}{
		{"the directory itself", []string{"pkg/service"}, "pkg/service", true},
		{"a nested directory", []string{"pkg/service"}, "pkg/service/billing", true},
		{"a file under the prefix", []string{"pkg/service"}, "pkg/service/billing/invoice.go", true},
		{"a sibling that only shares a name stem", []string{"pkg/service"}, "pkg/servicex", false},
		{"the root prefix contains every key", []string{"."}, "pkg/handlers", true},
		{"any one prefix of several is enough", []string{"pkg/db", "pkg/service"}, "pkg/service/billing", true},
		{"none of several prefixes contains the key", []string{"pkg/db", "pkg/service"}, "pkg/handlers", false},
		{"a layer with no prefixes contains nothing", nil, "pkg/handlers", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AnyLayerPrefixContains(tt.prefixes, tt.key); got != tt.want {
				t.Fatalf("AnyLayerPrefixContains(%q, %q) = %v, want %v", tt.prefixes, tt.key, got, tt.want)
			}
		})
	}
}
