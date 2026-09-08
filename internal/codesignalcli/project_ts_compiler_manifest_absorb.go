package codesignalcli

func (s *projectRootSummary) absorb(outcome projectRootOutcome) {
	s.rootFindings = append(s.rootFindings, ReadinessRootFinding{Root: outcome.root, Version: outcome.finding})
	if outcome.finding != "" {
		s.rootsWithFinding++
		s.findings = append(s.findings, outcome.finding)
	}
	if outcome.declaration != "" {
		s.declarations = append(s.declarations, rootDeclaration{root: outcome.root, declared: outcome.declaration})
	}
	s.ambiguous = s.ambiguous || outcome.ambiguous
	s.unreadable = s.unreadable || outcome.unreadable
	s.absorbDisqualified(outcome)
	s.absorbManifestDir(outcome)
}

func (s *projectRootSummary) absorbDisqualified(outcome projectRootOutcome) {
	if !outcome.disqualified {
		return
	}
	s.disqualified = true
	if s.rejectedDeclaration == "" {
		s.rejectedDeclaration = outcome.declaration
	}
}

func (s *projectRootSummary) absorbManifestDir(outcome projectRootOutcome) {
	if outcome.manifestDir == "" {
		return
	}
	if outcome.installed && s.installedDir == "" {
		s.installedDir = outcome.manifestDir
	}
	if s.firstManifestDir == "" {
		s.firstManifestDir = outcome.manifestDir
	}
}
