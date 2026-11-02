package projectmodel

import "testing"

func TestLayerBypassContainsDirMatchesTheLayerRulePrefixes(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		dir    string
		want   bool
	}{
		{name: "the directory itself", prefix: "pkg/service", dir: "pkg/service", want: true},
		{name: "a nested directory", prefix: "pkg/service", dir: "pkg/service/billing", want: true},
		{name: "a sibling that only shares a name stem", prefix: "pkg/service", dir: "pkg/servicex", want: false},
		{name: "the root prefix contains every directory", prefix: ".", dir: "pkg/handlers", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layer := BypassLayer{Name: "service", Prefixes: []string{tt.prefix}}

			if got := layerBypassContainsDir(layer, tt.dir); got != tt.want {
				t.Fatalf("layerBypassContainsDir(%q, %q) = %v, want %v", tt.prefix, tt.dir, got, tt.want)
			}
		})
	}
}
