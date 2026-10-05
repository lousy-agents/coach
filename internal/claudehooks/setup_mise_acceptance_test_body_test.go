package claudehooks

import (
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

func body_setupMiseAcceptanceTest_85(sub string, failing map[string]bool) string {
	if failing[sub] {
		return "1"
	}
	return "0"
}

func body_setupMiseAcceptanceTest_appendsThePATHExportOnlyOnce_149() {
	e := newHookEnv(pinnedMiseToml)
	log := filepath.Join(e.tmp, "mise-log")
	e.writeBin("mise", recordingMise(log, "2026.7.7", e.localBin, nil))

	for i := 0; i < 3; i++ {
		_, stderr, err := e.run(e.project)
		Expect(err).NotTo(HaveOccurred(), "run %d failed; stderr: %s", i, stderr)
	}

	var exports int
	for _, line := range strings.Split(e.envFileContents(), "\n") {
		if strings.HasPrefix(line, "export PATH=") {
			exports++
		}
	}
	Expect(exports).To(Equal(1), "CLAUDE_ENV_FILE must not accumulate duplicate PATH exports")
}
