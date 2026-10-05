package codesignalcli

import (
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// goBypassLayerFromConfig resolves config.RequiredLayer -- already validated
// by LoadProjectConfig to either be empty or name a declared layer -- into
// the projectmodel.BypassLayer BuildGoLayerBypass expects. The second return
// value is false when RequiredLayer is unset, in which case goProjectBackend
// skips the layer-bypass search entirely: BuildGoLayerBypass would otherwise
// treat a zero-value BypassLayer as ambiguous (see
// projectmodel.DiagLayerBypassAmbiguousLayer) and report incomplete coverage
// for a search the config never asked for.
func goBypassLayerFromConfig(config projectConfig) (projectmodel.BypassLayer, bool) {
	if config.RequiredLayer == "" {
		return projectmodel.BypassLayer{}, false
	}
	for _, layer := range config.Layers {
		if layer.Name == config.RequiredLayer {
			return projectmodel.BypassLayer{Name: layer.Name, Prefixes: append([]string(nil), layer.Prefixes...)}, true
		}
	}
	return projectmodel.BypassLayer{}, false
}

// combineProjectCoverage folds bypass (a LayerBypassResult's own Coverage)
// into model (the Go project model's own Coverage): Complete is ANDed, since
// either phase not completing means the combined result cannot claim a
// complete project analysis, and Diagnostics is unioned so bypass's own
// projectmodel-native diagnostics (e.g. a budget-exceeded or unresolved-root
// code) stay visible on the reported Coverage rather than being dropped when
// only one of the two phases actually failed. Phase/Counts/Budgets are kept
// from model: the two phases measure different dimensions (project-model
// build vs. one layer-bypass search), so merging their counts/budgets would
// conflate incomparable numbers rather than clarify anything.
func combineProjectCoverage(model, bypass projectmodel.Coverage) projectmodel.Coverage {
	combined := model
	combined.Complete = model.Complete && bypass.Complete
	if len(bypass.Diagnostics) > 0 {
		combined.Diagnostics = append(append([]projectmodel.Diagnostic(nil), model.Diagnostics...), bypass.Diagnostics...)
	}
	return combined
}
