package projectmodel

import "fmt"

func requiredLayerUnmatchedDiagnostic(layer BypassLayer) Diagnostic {
	return Diagnostic{
		Code:    DiagLayerBypassRequiredLayerUnmatched,
		Message: fmt.Sprintf("required_layer %q matched no package in the analysed program, so no layer-bypass search ran: correct its prefixes in the policy, or add the layer", layer.Name),
	}
}
