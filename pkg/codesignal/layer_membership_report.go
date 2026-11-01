package codesignal

import "github.com/lousy-agents/coach/pkg/domain"

// LayerMembershipReport is the report-level layer_membership block: how many
// project files under the configured roots sit outside every declared layer,
// per revision. It describes the reach of the layer rules and is information,
// not a finding: it adds no ProjectChange, diagnostic, or lifecycle effect.
//
// The block is present only when the count was computed. An absent block means
// "not computed" (no layers declared, an incomplete project model, or an
// analyzer that did not report enough to count); a present block with
// files_outside_layers 0 is a real zero. Both Go and TypeScript schema-2
// reports use it.
type LayerMembershipReport struct {
	Head LayerMembershipRevisionReport  `json:"head"`
	Base *LayerMembershipRevisionReport `json:"base,omitempty"`
}

// LayerMembershipRevisionReport is one revision's count, with the revision it
// describes.
type LayerMembershipRevisionReport struct {
	Revision           string `json:"revision"`
	AnalyzedFiles      int    `json:"analyzed_files"`
	FilesOutsideLayers int    `json:"files_outside_layers"`
}

// buildLayerMembership constructs the LayerMembershipReport from Input. It
// returns nil when the head count was not computed. The base side mirrors
// project_scope: it appears only when the base revision was analyzed.
func buildLayerMembership(input Input) *LayerMembershipReport {
	if input.HeadLayerMembership == nil {
		return nil
	}
	report := &LayerMembershipReport{
		Head: layerMembershipRevision(input.Scope.Revision, *input.HeadLayerMembership),
	}
	if input.ProjectBaseAnalyzed && input.BaseLayerMembership != nil {
		base := layerMembershipRevision(input.Scope.Base, *input.BaseLayerMembership)
		report.Base = &base
	}
	return report
}

func layerMembershipRevision(revision string, membership domain.LayerMembership) LayerMembershipRevisionReport {
	return LayerMembershipRevisionReport{
		Revision:           revision,
		AnalyzedFiles:      membership.AnalyzedFiles,
		FilesOutsideLayers: membership.FilesOutsideLayers,
	}
}
