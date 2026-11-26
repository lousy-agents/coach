package antipatternfixtures

import (
	"fmt"
	"strings"
)

const (
	godFilePath   = "orders/orders.go"
	checkBranches = 3
)

// godFile is #871's god file at the profile's branch points per file: one
// file of checks with three branches each, so the file holds exactly that
// many while no function is complex. Its fix gives each check its own file.
func godFile() Template {
	return Template{
		ID: "god-file", Name: "God file (overloaded file)", Status: "covered",
		observe: func(p profile) (observation, bool) {
			return typical(p.Shapes.BranchDensity, "branch points", densityThreshold, 40)
		},
		Bad:  godFileBad,
		Good: godFileGood,
		Expect: func(int) []Finding {
			return []Finding{{RuleID: "complexity.branch_density", Path: godFilePath}}
		},
	}
}

const orderType = "// Order is a customer's purchase.\ntype Order struct {\n\tQuantity int\n\tPrice    int\n}\n"

// checks are the check functions, one per three branch points, the last
// holding what remains.
func checks(branches int) []string {
	var functions []string
	for number := 1; branches > 0; number++ {
		count := min(branches, checkBranches)
		var body strings.Builder
		for branch := 1; branch <= count; branch++ {
			fmt.Fprintf(&body, "\tif o.Quantity > %d {\n\t\tscore++\n\t}\n", number*10+branch)
		}
		functions = append(functions, fmt.Sprintf("// Check%d scores one concern of an order.\nfunc Check%d(o Order) int {\n\tscore := 0\n%s\treturn score\n}\n", number, number, body.String()))
		branches -= count
	}
	return functions
}

func godFileBad(branches int) map[string]string {
	source := "package orders\n\n" + orderType + "\n// This file holds every check of an order: one reason to change per check, all in one place.\n\n" +
		strings.Join(checks(branches), "\n")
	return map[string]string{"go.mod": goMod("orders"), godFilePath: source}
}

func godFileGood(branches int) map[string]string {
	files := map[string]string{"go.mod": goMod("orders"), "orders/order.go": "package orders\n\n" + orderType}
	for i, function := range checks(branches) {
		files[fmt.Sprintf("orders/check%d.go", i+1)] = "package orders\n\n" + function
	}
	return files
}
