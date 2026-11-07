package sourcelayout_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const (
	repositoryPolicyPath = "../../.lousy-agents/coach.json"
	ringTablePath        = "../../docs/architecture/ADR-007-layered-ports-and-adapters.md"
)

// policyLayers are the rings the repository's own project-analysis policy places the top-level trees in.
type policyLayers struct {
	Layers []struct {
		Name     string   `json:"name"`
		Prefixes []string `json:"prefixes"`
	} `json:"layers"`
}

func readPolicyLayers(t *testing.T) policyLayers {
	t.Helper()
	body, err := os.ReadFile(repositoryPolicyPath)
	if err != nil {
		t.Fatalf("reading the repository policy: %v", err)
	}
	var policy policyLayers
	if err := json.Unmarshal(body, &policy); err != nil {
		t.Fatalf("decoding the repository policy: %v", err)
	}
	return policy
}

// ringTableRows maps each ring named in the first column of ADR-007's table of rings to the whole row.
func ringTableRows(t *testing.T) map[string]string {
	t.Helper()
	body, err := os.ReadFile(ringTablePath)
	if err != nil {
		t.Fatalf("reading ADR-007: %v", err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(string(body), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) >= 3 {
			rows[strings.TrimSpace(cells[1])] = line
		}
	}
	return rows
}

// The ring table in ADR-007 documents .lousy-agents/coach.json and places the same trees in the same rings. A tree
// the policy places in a ring and the table leaves out is one a reader of the ADR cannot place.
func TestEveryTreeTheRepositoryPolicyPlacesInARingIsInThatRingOfTheADRTable(t *testing.T) {
	rows := ringTableRows(t)
	for _, layer := range readPolicyLayers(t).Layers {
		row, listed := rows[layer.Name]
		if !listed {
			t.Errorf("ADR-007's ring table has no row for the %q ring that the repository policy defines", layer.Name)
			continue
		}
		for _, prefix := range missingFromRow(row, layer.Prefixes) {
			t.Errorf("the %q ring of ADR-007's table does not list %q, which the repository policy places there", layer.Name, prefix)
		}
	}
}

// missingFromRow are the trees the row does not name.
func missingFromRow(row string, prefixes []string) []string {
	var missing []string
	for _, prefix := range prefixes {
		if !listedInRow(row, prefix) {
			missing = append(missing, prefix)
		}
	}
	return missing
}

// listedInRow is whether the row names the tree: as it is, or, for a command under cmd/, by its name in the list
// that follows `cmd/`.
func listedInRow(row, prefix string) bool {
	if strings.Contains(row, "`"+prefix+"`") {
		return true
	}
	command, underCmd := strings.CutPrefix(prefix, "cmd/")
	return underCmd && strings.Contains(row, "`cmd/`") && strings.Contains(row, "`"+command+"`")
}
