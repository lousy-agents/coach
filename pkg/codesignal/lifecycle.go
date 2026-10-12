package codesignal

func lifecycleWithoutBase(baseline bool) Lifecycle {
	if baseline {
		return Lifecycle("baseline")
	}
	return Lifecycle("unknown")
}

func noBaseLifecycleForFile(fc FileChange, noBaseLifecycle Lifecycle) Lifecycle {
	if fc.Status == "added" && noBaseLifecycle != "baseline" {
		return "introduced"
	}
	return noBaseLifecycle
}

// classifyFileSignals computes Fingerprint, ID, and Lifecycle for every
// signal derived from one FileChange. noBaseLifecycle is the Lifecycle
// assigned to head-only signals when hasBase is false (e.g. "unknown" for
// a base-diff Report, "baseline" for a Repository Baseline Report).
func classifyFileSignals(hasBase bool, headSignals, baseSignals []Signal, noBaseLifecycle Lifecycle) []Signal {
	headGroups := groupAndOrder(headSignals)
	baseGroups := groupAndOrder(baseSignals)

	var result []Signal

	for _, k := range sortedKeys(headGroups) {
		headGroup := headGroups[k]
		baseGroup := baseGroups[k]
		nb := len(baseGroup)
		for i, sig := range headGroup {
			switch {
			case !hasBase:
				sig.Lifecycle = noBaseLifecycle
			case i < nb:
				sig.Lifecycle = "existing"
			case nb == 0:
				sig.Lifecycle = "introduced"
			default:
				sig.Lifecycle = "unknown"
			}
			sig.Fingerprint = computeFingerprint(sig.RuleID, sig.Path, sig.Subject, sig.Evidence, i)
			sig.ID = computeSignalID(sig.RuleID, sig.Path, sig.Subject, sig.Evidence, sig.Location.StartRow, sig.Location.StartCol, i)
			result = append(result, sig)
		}
	}

	for _, k := range sortedKeys(baseGroups) {
		result = appendResolvedBaseSignals(result, baseGroups[k], headGroups[k])
	}

	return result
}

func appendResolvedBaseSignals(result, baseGroup, headGroup []Signal) []Signal {
	nh := len(headGroup)
	for i, sig := range baseGroup {
		if i >= nh {
			sig.Lifecycle = "resolved"
			sig.Fingerprint = computeFingerprint(sig.RuleID, sig.Path, sig.Subject, sig.Evidence, i)
			sig.ID = computeSignalID(sig.RuleID, sig.Path, sig.Subject, sig.Evidence, sig.Location.StartRow, sig.Location.StartCol, i)
			result = append(result, sig)
		}
	}
	return result
}
