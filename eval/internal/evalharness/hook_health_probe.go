package evalharness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const probeName = "zz_probe"

// probePlan is the one file that makes a program break its own policy: written uncommitted, it must make the hook
// block with architecture.layer_violation at its path.
type probePlan struct {
	language string
	rel      string
	content  string
}

type probePolicy struct {
	Language string `json:"language"`
	Layers   []struct {
		Name     string   `json:"name"`
		Prefixes []string `json:"prefixes"`
	} `json:"layers"`
	ForbiddenImports []struct{ From, To string } `json:"forbidden_imports"`
}

// planProbe reads the program's own policy for its first forbidden edge and names a file in the from layer that
// imports a package of the to layer.
func planProbe(program string) (probePlan, error) {
	policy, err := readProbePolicy(program)
	if err != nil {
		return probePlan{}, err
	}
	if len(policy.ForbiddenImports) == 0 {
		return probePlan{}, fmt.Errorf("%w: %s: the policy names no forbidden import to probe", errHookHealthInput, program)
	}
	edge := policy.ForbiddenImports[0]
	from, to := layerDir(policy, edge.From), layerDir(policy, edge.To)
	if from == "" || to == "" {
		return probePlan{}, fmt.Errorf("%w: %s: the forbidden import %s -> %s names a layer the policy does not define", errHookHealthInput, program, edge.From, edge.To)
	}
	if policy.Language == "typescript" {
		return planTypeScriptProbe(program, from, to)
	}
	return planGoProbe(program, from, to)
}

func readProbePolicy(program string) (probePolicy, error) {
	var policy probePolicy
	content, err := os.ReadFile(filepath.Join(program, ".lousy-agents", "coach.json"))
	if err != nil {
		return policy, fmt.Errorf("%w: %s: no policy to read: %v", errHookHealthInput, program, err)
	}
	if err := json.Unmarshal(content, &policy); err != nil {
		return policy, fmt.Errorf("%w: %s: the policy is not valid JSON: %v", errHookHealthInput, program, err)
	}
	return policy, nil
}

func layerDir(policy probePolicy, name string) string {
	for _, layer := range policy.Layers {
		if layer.Name == name && len(layer.Prefixes) > 0 {
			return layer.Prefixes[0]
		}
	}
	return ""
}
