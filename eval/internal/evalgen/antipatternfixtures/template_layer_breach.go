package antipatternfixtures

import "fmt"

// dependencyRuleBreach is #871's dependency-rule breach across the profile's
// layers: the innermost layer's package imports the outermost layer's, which
// the policy forbids. Its fix leaves that import out.
func dependencyRuleBreach() Template {
	return Template{
		ID: "dependency-rule-breach", Name: "Dependency-rule breach", Status: "covered",
		observe: observeLayers,
		Bad:     breachBad,
		Good:    func(layers int) map[string]string { return layerBase(layers, "") },
		Padding: layerPadding,
		Expect: func(layers int) []Finding {
			inner := layerRoles[layers][layers-1]
			from, to := "internal/"+inner+"/"+layerPackages[0], "internal/api/"+layerPackages[0]
			return []Finding{{RuleID: "architecture.layer_violation", Path: from + "/reach.go", Subject: "architecture.layer_violation:" + from + "->" + to}}
		},
	}
}

func breachBad(layers int) map[string]string {
	files := layerBase(layers, "")
	inner := layerRoles[layers][layers-1]
	files[fmt.Sprintf("internal/%s/%s/reach.go", inner, layerPackages[0])] = "package " + layerPackages[0] + "\n\n" +
		"import outer \"example.com/layers/internal/api/" + layerPackages[0] + "\"\n\n" +
		"// Reach names the outermost layer's package from the innermost one, against the dependency rule: source\n" +
		"// dependencies point inward, so the inner layer cannot be used or tested without the outer one.\n" +
		"func Reach() string {\n\treturn outer.Describe()\n}\n"
	return files
}
