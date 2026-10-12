package codesignal

func processHeadResult(fc FileChange) ([]Diagnostic, []Signal) {
	if fc.Head == nil {
		if fc.Status == "modified" || fc.Status == "added" {
			return []Diagnostic{{
				Path:    fc.Path,
				Kind:    "missing_head_result",
				Message: "file change status \"" + string(fc.Status) + "\" has no head analysis result",
			}}, nil
		}
		return nil, nil
	}

	switch fc.Head.ParseStatus {
	case "ok":
		counts := findingCountsByKind(fc.Head.Findings)
		signals := signalsFromFindings(fc.Path, fc.Head.Findings, counts)
		signals = append(signals, signalsFromMetrics(fc.Path, fc.Head.Metrics)...)
		signals = append(signals, signalsFromImports(fc.Path, fc.Head.Language, fc.Head.Imports)...)
		signals = append(signals, signalsFromCognitiveComplexity(fc.Path, fc.Head.CognitiveComplexity)...)
		signals = append(signals, signalsFromReactOrchestration(fc.Path, fc.Head.ReactComponents)...)
		return nil, signals
	case "syntax_errors":
		diagnostics := make([]Diagnostic, 0, len(fc.Head.SyntaxErrors))
		for _, issue := range fc.Head.SyntaxErrors {
			location := issue.Location
			diagnostics = append(diagnostics, Diagnostic{
				Path:     fc.Path,
				Kind:     "syntax_errors",
				Location: &location,
				Message:  "head analysis found a syntax issue of kind \"" + issue.Kind + "\"",
			})
		}
		return diagnostics, nil
	default:
		return []Diagnostic{{
			Path:    fc.Path,
			Kind:    "unsupported_parse_status",
			Message: "head analysis result has unsupported parse status \"" + string(fc.Head.ParseStatus) + "\"",
		}}, nil
	}
}

func extractBaseSignals(fc FileChange) []Signal {
	if !baseUsableForLifecycle(fc) || fc.Base.ParseStatus != "ok" {
		return nil
	}
	counts := findingCountsByKind(fc.Base.Findings)
	signals := signalsFromFindings(fc.Path, fc.Base.Findings, counts)
	signals = append(signals, signalsFromMetrics(fc.Path, fc.Base.Metrics)...)
	signals = append(signals, signalsFromImports(fc.Path, fc.Base.Language, fc.Base.Imports)...)
	signals = append(signals, signalsFromCognitiveComplexity(fc.Path, fc.Base.CognitiveComplexity)...)
	signals = append(signals, signalsFromReactOrchestration(fc.Path, fc.Base.ReactComponents)...)
	return signals
}
