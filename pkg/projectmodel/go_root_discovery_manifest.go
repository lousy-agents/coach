package projectmodel

import (
	"path"

	"golang.org/x/mod/modfile"
)

// recordGoFile parses the go.mod/go.work file already read at p (data) and
// records the successful module/workspace, or a DiagRootInvalid diagnostic
// on parse failure. base must be "go.mod" or "go.work"; any other value is a
// no-op, since the caller only invokes this after that check.
func (d *goProjectDiscovery) recordGoFile(p, base string, data []byte) {
	dir := path.Dir(p)
	switch base {
	case "go.mod":
		mf, parseErr := modfile.Parse(p, data, nil)
		if parseErr != nil {
			d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagRootInvalid, Path: p, Message: parseErr.Error()})
			d.ModulesSkipped++
			return
		}
		d.Modules[dir] = mf
	case "go.work":
		wf, parseErr := modfile.ParseWork(p, data, nil)
		if parseErr != nil {
			d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagRootInvalid, Path: p, Message: parseErr.Error()})
			return
		}
		d.Workspaces[dir] = wf
	}
}
