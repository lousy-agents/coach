package main

import (
	"encoding/json"
	"io/fs"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// scanPorts are the collaborators a scan uses to find and read a policy, to pick
// the backend that analyzes it, and to run an optional step after a successful
// scan. run wires the real ones in; a spec that needs a policy, a backend or an
// answer a real repository cannot provide passes its own instead of replacing a
// package variable.
type scanPorts struct {
	discover       func(dir, revision string) (projectconfig.Selection, error)
	loadExplicit   func(dir, revision, repoPath string) (json.RawMessage, error)
	loadDiscovered func(dir, revision, repoPath string) (json.RawMessage, error)
	resolveBackend func(language string) error
	lookupBackend  func(language string) codesignalcli.ProjectBackend
	// optionalPreparation runs after a successful scan and before its report is
	// rendered, so a step the customer may decline or cancel can attach to it.
	optionalPreparation func(dir string, f codesignalFlags, stdout, stderr *os.File) optionalPreparationResult
	// discoverTSRoots lists the TypeScript roots of a snapshot for guided policy
	// authoring. A spec supplies its own to produce a discovery result a real Git
	// snapshot never can, such as an unavailable root.
	discoverTSRoots func(snapshot fs.FS, budgets projectmodel.GoBudgets) (projectmodel.TSRootDiscoveryResult, error)
}

func defaultScanPorts() scanPorts {
	return scanPorts{
		discover:            projectconfig.Discover,
		loadExplicit:        projectconfig.LoadExplicit,
		loadDiscovered:      projectconfig.Load,
		resolveBackend:      codesignalcli.ResolveProjectBackend,
		lookupBackend:       lookupRegisteredBackend,
		optionalPreparation: noOptionalPreparation,
		discoverTSRoots:     projectmodel.DiscoverTSRoots,
	}
}

func noOptionalPreparation(string, codesignalFlags, *os.File, *os.File) optionalPreparationResult {
	return optionalPreparationResult{}
}

func lookupRegisteredBackend(language string) codesignalcli.ProjectBackend {
	switch language {
	case "go":
		return codesignalcli.NewGoProjectBackend()
	case "typescript":
		return codesignalcli.NewTSProjectBackend()
	default:
		return nil
	}
}

// loader is the function that reads the policy a selection names: an explicit
// --project-config is read through the on-disk check, a discovered one at the
// revision.
func (p scanPorts) loader(explicit bool) func(dir, revision, repoPath string) (json.RawMessage, error) {
	if explicit {
		return p.loadExplicit
	}
	return p.loadDiscovered
}
