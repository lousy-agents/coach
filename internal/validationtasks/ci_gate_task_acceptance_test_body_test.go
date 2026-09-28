package validationtasks

import (
	. "github.com/onsi/gomega"
)

func body_ciGateTaskAcceptanceTest_runsNoTests_73(toml string) {
	body := taskSteps(toml, "ci-gate")
	Expect(body).NotTo(BeEmpty(), "ci-gate must exist for these exclusions to mean anything")
	for _, testTask := range []string{"test", "test-examples", "ci-go", "ci", "ci-fast", "ci-all", "js-ci", "projectmodel-sidecar-acceptance"} {
		Expect(body).NotTo(ContainSubstring(`task = "`+testTask+`"`),
			"%q executes tests; the gate must stay cheap enough that it is never the reason a PR is slow", testTask)
	}
	Expect(body).NotTo(ContainSubstring("go test"),
		"a raw go test line evades the task-name exclusions above")
}

func body_ciGateTaskAcceptanceTest_stillCatchesTheCheapMechanicalBreaks_97(toml string) {
	body := taskSteps(toml, "ci-gate")
	for _, step := range []string{"gofmt", "go-vet", "acceptance-style-check"} {
		Expect(indexOfStep(body, step)).To(BeNumerically(">", -1),
			"%q costs seconds and is the most common way a PR comes back red; losing it makes the gate pure overhead", step)
	}
}
