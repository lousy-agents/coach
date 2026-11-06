package main

import "github.com/lousy-agents/coach/pkg/codesignal"

func lifecyclesAt(report *codesignal.Report, path string) []codesignal.Lifecycle {
	lifecycles := []codesignal.Lifecycle{}
	for _, signal := range report.Signals {
		if signal.Path == path {
			lifecycles = append(lifecycles, signal.Lifecycle)
		}
	}
	return lifecycles
}
