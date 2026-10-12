package main

func gapCodes(doc readinessResultDoc) []string {
	codes := make([]string, len(doc.Gaps))
	for i, g := range doc.Gaps {
		codes[i] = g.Code
	}
	return codes
}

func nextActionKinds(doc readinessResultDoc) []string {
	kinds := make([]string, len(doc.NextActions))
	for i, a := range doc.NextActions {
		kinds[i] = a.Kind
	}
	return kinds
}

type readinessRootFindingDoc struct {
	Root    string `json:"root"`
	Version string `json:"version"`
}

type readinessCheckDoc struct {
	State             string                    `json:"state"`
	Code              string                    `json:"code"`
	Kind              string                    `json:"kind"`
	Version           string                    `json:"version"`
	ExpectedVersion   string                    `json:"expected_version"`
	FoundVersion      string                    `json:"found_version"`
	PinnedVersion     string                    `json:"pinned_version"`
	SupportedVersions []string                  `json:"supported_versions"`
	RootFindings      []readinessRootFindingDoc `json:"root_findings"`
	Origin            string                    `json:"origin"`
	Detail            string                    `json:"detail"`
}

type readinessResultDoc struct {
	SchemaVersion string `json:"schema_version"`
	Status        string `json:"status"`
	Language      string `json:"language"`
	Revision      string `json:"revision"`
	DirtyWorktree struct {
		RelevantChanges bool     `json:"relevant_changes"`
		Paths           []string `json:"paths"`
	} `json:"dirty_worktree"`
	Checks struct {
		ProjectShape   readinessCheckDoc `json:"project_shape"`
		Policy         readinessCheckDoc `json:"policy"`
		Node           readinessCheckDoc `json:"node"`
		Runtime        readinessCheckDoc `json:"runtime"`
		Compiler       readinessCheckDoc `json:"compiler"`
		PackageManager readinessCheckDoc `json:"package_manager"`
	} `json:"checks"`
	Gaps []struct {
		Code               string `json:"code"`
		PackageManagerKind string `json:"package_manager_kind"`
	} `json:"gaps"`
	Warnings []struct {
		Code              string `json:"code"`
		DeclaredVersion   string `json:"declared_version"`
		FoundVersion      string `json:"found_version"`
		DeclarationOrigin string `json:"declaration_origin"`
		Root              string `json:"root"`
	} `json:"warnings"`
	NextActions []readinessNextActionDoc `json:"next_actions"`
}

type readinessNextActionDoc struct {
	Kind               string   `json:"kind"`
	Executable         bool     `json:"executable"`
	RuntimeKind        string   `json:"runtime_kind"`
	PackageManagerKind string   `json:"package_manager_kind"`
	Supported          []string `json:"supported"`
	FoundVersion       string   `json:"found_version"`
	Detail             string   `json:"detail"`
	Choices            []string `json:"choices"`
}

func gapEntries(doc readinessResultDoc) []string {
	entries := make([]string, len(doc.Gaps))
	for i, g := range doc.Gaps {
		if g.PackageManagerKind == "" {
			entries[i] = g.Code
			continue
		}
		entries[i] = g.Code + ":" + g.PackageManagerKind
	}
	return entries
}

// nextActionOfKind returns the last next action of kind, and whether any was
// present.
func nextActionOfKind(doc readinessResultDoc, kind string) (readinessNextActionDoc, bool) {
	var action readinessNextActionDoc
	found := false
	for _, a := range doc.NextActions {
		if a.Kind == kind {
			action, found = a, true
		}
	}
	return action, found
}
