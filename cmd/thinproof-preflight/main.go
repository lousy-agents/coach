// Command thinproof-preflight is the no-pull/offline Compose preflight for
// issue #79's Task 0.3 thin offline proof (see
// docs/architecture/acceptance-harness.md section 2's binding contract): it
// checks, via `docker image inspect`, that the two local images
// deploy/compose/thinproof's docker-compose.yml expects already exist,
// before any `docker compose up` runs. A missing image fails loudly here,
// naming the exact image and the one-time online acquisition step, rather
// than surfacing as an opaque mid-run Compose failure.
package main

import (
	"fmt"
	"os"
	"os/exec"
)

// requiredImages are the image tags deploy/compose/thinproof/docker-compose.yml
// builds and runs with pull_policy: never.
var requiredImages = []string{
	"coach/fakegithub-thinproof:0.1.0",
	"coach/thinproof-runner:0.1.0",
}

func main() {
	var missing []string
	for _, image := range requiredImages {
		if !imageExists(image) {
			missing = append(missing, image)
		}
	}

	if len(missing) == 0 {
		return
	}

	fmt.Fprintln(os.Stderr, "thinproof-preflight: refusing to run -- missing local Docker image(s):")
	for _, image := range missing {
		fmt.Fprintf(os.Stderr, "  - %s\n", image)
	}
	fmt.Fprintln(os.Stderr, "run `mise run thinproof-build` once, online, to build these images locally, then rerun `mise run test-acceptance-thin-proof` offline.")
	os.Exit(1)
}

func imageExists(image string) bool {
	cmd := exec.Command("docker", "image", "inspect", image)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}
