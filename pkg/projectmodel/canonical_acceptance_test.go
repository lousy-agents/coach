package projectmodel_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var _ = Describe("canonical project-model wire shape", func() {
	It("uses revision identity and repo-relative node fields without filesystem roots", func() {
		model := projectmodel.Model{
			SchemaVersion: projectmodel.SchemaVersion,
			Snapshot: projectmodel.Snapshot{
				Revision:      "head-sha",
				TreeID:        "tree-sha",
				ConfigDigest:  "cfg-digest",
				BackendDigest: "backend-digest",
			},
			Workspaces: []projectmodel.Workspace{{
				ID:       "workspace:.",
				Language: "go",
				Root:     ".",
				Projects: []string{"module:."},
			}},
			Modules: []projectmodel.Module{{
				ID:       "module:.",
				Path:     ".",
				Language: "go",
				Files:    []string{"a.go"},
			}},
			Packages: []projectmodel.Package{{
				ID:       "package:a",
				Path:     "a",
				Language: "go",
				Files:    []string{"a.go"},
			}},
			Files:    []projectmodel.File{{ID: "file:a.go", Path: "a.go", Language: "go"}},
			Coverage: projectmodel.Coverage{Phase: "complete", Complete: true},
		}

		encoded, err := json.Marshal(model)
		Expect(err).NotTo(HaveOccurred())

		var document map[string]json.RawMessage
		Expect(json.Unmarshal(encoded, &document)).To(Succeed())
		Expect(document).To(HaveKey("snapshot"))
		Expect(document).To(HaveKey("workspaces"))
		Expect(document).To(HaveKey("modules"))
		Expect(document).To(HaveKey("packages"))
		Expect(document).To(HaveKey("files"))

		var snapshot map[string]json.RawMessage
		Expect(json.Unmarshal(document["snapshot"], &snapshot)).To(Succeed())
		Expect(snapshot).To(HaveKey("revision"))
		Expect(snapshot).To(HaveKey("tree_id"))
		Expect(snapshot).To(HaveKey("config_digest"))
		Expect(snapshot).To(HaveKey("backend_digest"))
		Expect(snapshot).NotTo(HaveKey("root"))

		var workspaces []map[string]json.RawMessage
		Expect(json.Unmarshal(document["workspaces"], &workspaces)).To(Succeed())
		Expect(workspaces).To(HaveLen(1))
		Expect(workspaces[0]).To(HaveKey("id"))
		Expect(workspaces[0]).To(HaveKey("language"))
		Expect(workspaces[0]).To(HaveKey("root"))
		Expect(workspaces[0]).To(HaveKey("projects"))
		Expect(workspaces[0]).NotTo(HaveKey("path"))
		Expect(workspaces[0]).NotTo(HaveKey("files"))

		var modules []map[string]json.RawMessage
		Expect(json.Unmarshal(document["modules"], &modules)).To(Succeed())
		Expect(modules[0]).To(HaveKey("id"))
		Expect(modules[0]).To(HaveKey("path"))
		Expect(modules[0]).To(HaveKey("language"))
		Expect(modules[0]).To(HaveKey("files"))

		var files []map[string]json.RawMessage
		Expect(json.Unmarshal(document["files"], &files)).To(Succeed())
		Expect(files[0]).To(HaveKey("id"))
		Expect(files[0]).To(HaveKey("path"))
		Expect(files[0]).To(HaveKey("language"))
	})

	It("uses code rather than a policy-shaped kind for fact diagnostics", func() {
		encoded, err := json.Marshal(projectmodel.Coverage{
			Phase:    "imports",
			Complete: false,
			Diagnostics: []projectmodel.Diagnostic{{
				Code:    "unresolved_import",
				Message: "could not resolve module",
			}},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(encoded)).To(ContainSubstring(`"code":"unresolved_import"`))
		Expect(string(encoded)).NotTo(ContainSubstring(`"kind"`))
	})
})
