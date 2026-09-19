package codesignalcli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDetectBunfigHazard pins the message each hazard class produces, not
// merely that some hazard was reported. The acceptance specs assert the gap
// code (package_manager_config_unverifiable), which the catch-all keyword
// check below produces for any file mentioning "install" or "registry" --
// so they pass identically whether the redirect is recognized precisely or
// only suspected, and cannot tell the two apart. A reader of the withheld
// choice's reason can.
func TestDetectBunfigHazard(t *testing.T) {
	const unverifiable = "committed bunfig.toml could not be verified to leave package resolution unredirected"

	for _, tc := range []struct {
		name     string
		contents string
		want     string
	}{
		{"no file at all", "", ""},
		{"install registry redirect", "[install]\nregistry = \"https://mirror.example.invalid/npm/\"\n", `committed bunfig.toml redirects the install registry (registry=https://mirror.example.invalid/npm/)`},
		{"install scopes redirect", "[install]\nscopes = { \"@acme\" = \"https://mirror.example.invalid/\" }\n", `committed bunfig.toml redirects scoped install registries (scopes={ "@acme" = "https://mirror.example.invalid/" })`},
		{"scoped registry table", "[install.scopes]\n\"@example\" = \"https://mirror.example.invalid/npm/\"\n", `committed bunfig.toml redirects a scoped install registry ("@example"=https://mirror.example.invalid/npm/)`},
		{"header with a trailing comment still selects the section", "[install] # mirror\nregistry = \"https://mirror.example.invalid/\"\n", `committed bunfig.toml redirects the install registry (registry=https://mirror.example.invalid/)`},
		{"quoted, spaced header still selects the section", "[ \"install\" ]\nregistry = \"https://mirror.example.invalid/\"\n", `committed bunfig.toml redirects the install registry (registry=https://mirror.example.invalid/)`},
		{"a UTF-8 BOM does not hide the first section header", "\xEF\xBB\xBF[install]\nregistry = \"https://mirror.example.invalid/\"\n", `committed bunfig.toml redirects the install registry (registry=https://mirror.example.invalid/)`},
		{"an escape sequence anywhere is unverifiable", "telemetry = false\nlogLevel = \"deb\\u0075g\"\n", unverifiable},
		{"a resolution keyword without a recognized redirect is unverifiable", "[install]\nexact = true\n", unverifiable},
		{"a file naming nothing about resolution is no hazard", "telemetry = false\n# a comment\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.contents != "" {
				if err := os.WriteFile(filepath.Join(root, "bunfig.toml"), []byte(tc.contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := detectBunfigHazard(root); got != tc.want {
				t.Fatalf("detectBunfigHazard() = %q, want %q", got, tc.want)
			}
		})
	}
}
