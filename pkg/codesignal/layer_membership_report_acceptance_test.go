package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/domain"
)

var _ = Describe("layer_membership report block when only one revision's count was computed", func() {
	diffInput := func(head, base *domain.LayerMembership) codesignal.Input {
		return codesignal.Input{
			Scope:               codesignal.Scope{Revision: "head-rev", Base: "base-rev"},
			ProjectBaseAnalyzed: true,
			HeadLayerMembership: head,
			BaseLayerMembership: base,
		}
	}
	options := codesignal.Options{ProjectEnabled: true}

	When("both revisions have a count", func() {
		It("reports the head and the base side with their revisions", func() {
			report := build(options, diffInput(
				&domain.LayerMembership{AnalyzedFiles: 3, FilesOutsideLayers: 1},
				&domain.LayerMembership{AnalyzedFiles: 2, FilesOutsideLayers: 0}))

			Expect(report.LayerMembership).NotTo(BeNil())
			Expect(report.LayerMembership.Head).To(Equal(codesignal.LayerMembershipRevisionReport{Revision: "head-rev", AnalyzedFiles: 3, FilesOutsideLayers: 1}))
			Expect(report.LayerMembership.Base).To(Equal(&codesignal.LayerMembershipRevisionReport{Revision: "base-rev", AnalyzedFiles: 2, FilesOutsideLayers: 0}))
		})
	})

	When("the head count was not computed but the base count was", func() {
		It("omits the whole block, because the block describes the head's reach and a lone base line would read as the current state", func() {
			report := build(options, diffInput(nil, &domain.LayerMembership{AnalyzedFiles: 2, FilesOutsideLayers: 1}))

			Expect(report.LayerMembership).To(BeNil())
		})
	})

	When("the base count was not computed but the head count was", func() {
		It("reports the head side alone", func() {
			report := build(options, diffInput(&domain.LayerMembership{AnalyzedFiles: 3, FilesOutsideLayers: 1}, nil))

			Expect(report.LayerMembership).NotTo(BeNil())
			Expect(report.LayerMembership.Base).To(BeNil())
		})
	})
})
