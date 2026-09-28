package projectmodel

import "github.com/lousy-agents/coach/pkg/domain"

// Fact types and their stable codes are defined in pkg/domain. These aliases
// keep this package's names stable for existing callers. pkg/codesignal
// depends on pkg/domain directly, so the use-case layer does not import this
// adapter.

const SchemaVersion = domain.SchemaVersion

const (
	DiagBackendUnavailable                            = domain.DiagBackendUnavailable
	DiagRootScopeIncomplete                           = domain.DiagRootScopeIncomplete
	ReachabilityAlgorithm                             = domain.ReachabilityAlgorithm
	TSReachabilityAlgorithm                           = domain.TSReachabilityAlgorithm
	KindPossibleCallReachability                      = domain.KindPossibleCallReachability
	DiagReachabilityBudgetExceeded                    = domain.DiagReachabilityBudgetExceeded
	DiagReachabilitySourceLoadFailed                  = domain.DiagReachabilitySourceLoadFailed
	LayerBypassAlgorithm                              = domain.LayerBypassAlgorithm
	TSLayerBypassAlgorithm                            = domain.TSLayerBypassAlgorithm
	DiagLayerBypassBudgetExceeded                     = domain.DiagLayerBypassBudgetExceeded
	DiagLayerBypassSourceLoadFailed                   = domain.DiagLayerBypassSourceLoadFailed
	DiagLayerBypassAmbiguousLayer                     = domain.DiagLayerBypassAmbiguousLayer
	InclusionRuleTSConfigIncludesNoTestClassification = domain.InclusionRuleTSConfigIncludesNoTestClassification
)

type (
	Coverage               = domain.Coverage
	Diagnostic             = domain.Diagnostic
	ReachabilityConfidence = domain.ReachabilityConfidence
	ReachabilityStep       = domain.ReachabilityStep
	ReachabilityFact       = domain.ReachabilityFact
	ReachabilityResult     = domain.ReachabilityResult
	LayerBypassConfidence  = domain.LayerBypassConfidence
	LayerBypassStep        = domain.LayerBypassStep
	BypassLayer            = domain.BypassLayer
	LayerBypassWitness     = domain.LayerBypassWitness
	LayerBypassResult      = domain.LayerBypassResult
	ProjectScopeRoot       = domain.ProjectScopeRoot
	ProjectScope           = domain.ProjectScope
	Model                  = domain.Model
	Snapshot               = domain.Snapshot
	Workspace              = domain.Workspace
	Module                 = domain.Module
	Package                = domain.Package
	File                   = domain.File
	ImportEdge             = domain.ImportEdge
	CallFact               = domain.CallFact
	RootScope              = domain.RootScope
)

const ReachabilityConfidenceResolvedDirect = domain.ReachabilityConfidenceResolvedDirect

const LayerBypassConfidenceHigh = domain.LayerBypassConfidenceHigh

func canonicalCoverage(in Coverage) Coverage { return domain.CanonicalCoverage(in) }

func canonicalCallFacts(in []CallFact) []CallFact { return domain.CanonicalCallFacts(in) }
