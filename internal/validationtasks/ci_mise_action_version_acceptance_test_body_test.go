package validationtasks

import (
	"strings"

	. "github.com/onsi/gomega"
)

func body_ciMiseActionVersionAcceptanceTest_failsIfAnyJdxMiseActionStepOmitsAVersionInputEqu_28(yml string, toml string) {
	minVersion := miseTomlMinVersion(toml)
	Expect(minVersion).NotTo(BeEmpty(),
		"mise.toml must declare min_version so the CI pin has a source of truth")

	steps := miseActionStepBodies(yml)
	Expect(steps).NotTo(BeEmpty(),
		"ci.yml must contain jdx/mise-action steps or this spec cannot catch an unpinned install")

	var unpinned []string
	for _, step := range steps {
		version, ok := miseActionVersion(step)
		if !ok || version != minVersion {
			unpinned = append(unpinned, step)
		}
	}
	Expect(unpinned).To(BeEmpty(),
		"%d jdx/mise-action step(s) omit a version input equal to mise.toml min_version %q:\n%s",
		len(unpinned), minVersion, strings.Join(unpinned, "\n\n"))
}
