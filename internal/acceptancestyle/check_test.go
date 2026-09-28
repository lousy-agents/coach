package acceptancestyle

import (
	"testing"
)

func TestCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		files          map[string]string
		wantViolations int
		wantPathSubstr string
	}{
		{
			name: "acceptance file with ginkgo import and RunSpecs is OK",
			files: map[string]string{
				"pkg/a/a_acceptance_test.go": `package a_test
import (
	"testing"
	. "github.com/onsi/ginkgo/v2"
)
func TestAAcceptance(t *testing.T) { RunSpecs(t, "a") }
`,
			},
			wantViolations: 0,
		},
		{
			name: "blank ginkgo import without specs is a violation",
			files: map[string]string{
				"pkg/blank/blank_acceptance_test.go": `package blank_test
import (
	"testing"
	_ "github.com/onsi/ginkgo/v2"
)
func TestBlankAcceptance(t *testing.T) {}
`,
			},
			wantViolations: 1,
			wantPathSubstr: "blank_acceptance_test.go",
		},
		{
			name: "ginkgo import without Describe/It/RunSpecs is a violation",
			files: map[string]string{
				"pkg/nospec/nospec_acceptance_test.go": `package nospec_test
import (
	"testing"
	. "github.com/onsi/ginkgo/v2"
)
func TestNoSpecAcceptance(t *testing.T) {}
`,
			},
			wantViolations: 1,
			wantPathSubstr: "nospec_acceptance_test.go",
		},
		{
			name: "acceptance file without ginkgo is a violation",
			files: map[string]string{
				"pkg/b/b_acceptance_test.go": `package b_test
import "testing"
func TestBAcceptance(t *testing.T) {}
`,
			},
			wantViolations: 1,
			wantPathSubstr: "b_acceptance_test.go",
		},
		{
			name: "non-acceptance test file is ignored",
			files: map[string]string{
				"pkg/c/c_test.go": `package c_test
import "testing"
func TestC(t *testing.T) {}
`,
			},
			wantViolations: 0,
		},
		{
			name: "non-allowlisted bare acceptance_test.go without ginkgo is a violation",
			files: map[string]string{
				"pkg/other/acceptance_test.go": `package other_test
import "testing"
func TestOtherAcceptance(t *testing.T) {}
`,
			},
			wantViolations: 1,
			wantPathSubstr: "acceptance_test.go",
		},
		{
			name: "allowlisted bare acceptance_test.go without ginkgo is OK",
			files: map[string]string{
				// Content lacks ginkgo: only allowlist keeps this from violating.
				"internal/acceptanceharness/queueconformance/acceptance_test.go": `package queueconformance
import "testing"
func TestQueueConformanceAcceptance(t *testing.T) {}
`,
			},
			wantViolations: 0,
		},
		{
			name: "allowlisted compose_acceptance_test.go without ginkgo is OK",
			files: map[string]string{
				"internal/acceptanceharness/thinproof/compose_acceptance_test.go": `package thinproof
import "testing"
func TestComposeAcceptance(t *testing.T) {}
`,
			},
			wantViolations: 0,
		},
		{
			name: "skips vendor tree",
			files: map[string]string{
				"vendor/x/x_acceptance_test.go": `package x
import "testing"
func TestXAcceptance(t *testing.T) {}
`,
			},
			wantViolations: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			body_checkTest_127(t, tt)
		})
	}
}
