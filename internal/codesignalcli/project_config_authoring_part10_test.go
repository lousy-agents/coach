package codesignalcli

import (
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_ApprovalGateRequiresExactApprovalToken(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	declineCases := []struct {
		name   string
		answer string
	}{
		{name: "declines with a blank answer", answer: ""},
		{name: "declines with an unrelated word", answer: "no"},
		{name: "declines with a near-miss token", answer: "approved"},
	}

	for _, tc := range declineCases {
		t.Run(tc.name, func(t *testing.T) {
			body_projectConfigAuthoringPart10Test_24(t, discovered, tc)
		})
	}

	t.Run("approves with the exact approval token", func(t *testing.T) {
		body_projectConfigAuthoringPart10Test_approvesWithTheExactApprovalToken_44(t, discovered)
	})

	t.Run("approves case-insensitively", func(t *testing.T) {
		body_projectConfigAuthoringPart10Test_approvesCaseInsensitively_58(t, discovered)
	})
}

// TestAuthorProjectConfig_ExhaustedInputCancelsInsteadOfSpinning reproduces the
// EOF-on-a-retry-prompt hang: an invalid answer at each of the four
// layer/forbidden-pair/required-layer stages sends the user to
// promptRetryOrCancel, and the caller never types "cancel" -- input simply
// runs out. Each case must terminate as a cancellation, not spin forever
// re-reading "" from an exhausted reader. Every case's lines lead with a
// valid root answer ("."): root selection now validates and retries/cancels
// on its own (see
// TestAuthorProjectConfig_RootSelectionRejectsInvalidOrEmptyAndOffersRetryOrCancel),
// so a leading "" (once accepted as "select no roots") would otherwise be
// consumed and rejected by that stage instead of ever reaching the
// layer/forbidden-pair/required-layer stage the case is named for. Each
// case's wantSubstr pins the stage-specific rejection text in the transcript
// so a future change to collection order cannot silently re-shift the
// sequence again without a test noticing.
func TestAuthorProjectConfig_ExhaustedInputCancelsInsteadOfSpinning(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Complete: true}
	const watchdog = 3 * time.Second

	declaredDomainLayer := []projectconfig.Layer{{Name: "domain", Prefixes: []string{"internal/domain"}}}

	cases := []struct {
		name       string
		lines      []string
		wantSubstr string
		// wantLayers pins the layer list already accepted by the time the
		// case's own stage rejects its answer -- catching a regression where
		// an earlier stage silently consumes what was meant to be this
		// case's declared "domain" layer, leaving zero layers declared
		// instead (the case's own rejection text can otherwise stay
		// identical either way, e.g. an undeclared-layer reference is
		// undeclared whether zero or one other layer exists).
		wantLayers []projectconfig.Layer
	}{
		{
			name:       "invalid (blank) layer prefixes, then input runs out at the retry prompt",
			lines:      []string{".", "domain"},
			wantSubstr: `layer "domain" must contain at least one prefix`,
			wantLayers: nil,
		},
		{
			name:       "duplicate layer name, then input runs out at the retry prompt",
			lines:      []string{".", "domain", "internal/domain", "domain"},
			wantSubstr: `layer name "domain" is already used`,
			wantLayers: declaredDomainLayer,
		},
		{
			name:       "forbidden pair referencing an undeclared layer, then input runs out at the retry prompt",
			lines:      []string{".", "domain", "internal/domain", "", "unknown", "domain"},
			wantSubstr: `forbidden import pair references undefined layer "unknown"`,
			wantLayers: declaredDomainLayer,
		},
		{
			name:       "required layer naming an undeclared layer, then input runs out at the retry prompt",
			lines:      []string{".", "domain", "internal/domain", "", "", "unknown"},
			wantSubstr: `required_layer references undefined layer "unknown"`,
			wantLayers: declaredDomainLayer,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body_projectConfigAuthoringPart10Test_134(t, discovered, watchdog, tc)
		})
	}
}
