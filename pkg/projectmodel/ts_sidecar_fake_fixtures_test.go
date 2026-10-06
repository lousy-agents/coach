package projectmodel_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing/fstest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var fakeTSSidecarPath string

var _ = BeforeSuite(func() {
	dir, err := os.MkdirTemp("", "fake-ts-sidecar-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	fakeTSSidecarPath = filepath.Join(dir, "fake-ts-sidecar")
	build := exec.Command("go", "build", "-o", fakeTSSidecarPath, "./testdata/fake_ts_sidecar")
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building fake ts sidecar: %s", output)
})

func tsSidecarSnapshot() fstest.MapFS {
	return fstest.MapFS{
		"src/a.ts":     &fstest.MapFile{Data: []byte("import { b } from './b';\nexport const a = b;\n")},
		"src/b.ts":     &fstest.MapFile{Data: []byte("export const b = 1;\n")},
		"src/c.tsx":    &fstest.MapFile{Data: []byte("export const C = () => null;\n")},
		"src/notes.md": &fstest.MapFile{Data: []byte("not a TypeScript source file\n")},
	}
}

func sidecarOptsWithMode(mode string) projectmodel.TSSidecarOptions {
	return projectmodel.TSSidecarOptions{
		BinaryPath: fakeTSSidecarPath,
		Args:       []string{"--mode=" + mode},
		Timeout:    5 * time.Second,
	}
}

// sidecarOptsWithModeAndCounter extends sidecarOptsWithMode with the fake
// sidecar's --invocation-counter-file flag, letting a spec assert exactly
// how many separate subprocess round trips a call sequence made.
func sidecarOptsWithModeAndCounter(mode, counterFile string) projectmodel.TSSidecarOptions {
	opts := sidecarOptsWithMode(mode)
	opts.Args = append(opts.Args, "--invocation-counter-file="+counterFile)
	return opts
}

// invocationCount reads the fake sidecar's invocation counter file (one line
// appended per subprocess invocation) and reports how many invocations it
// recorded. A missing file (no invocation yet) counts as zero.
func invocationCount(counterFile string) int {
	data, err := os.ReadFile(counterFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		Fail(fmt.Sprintf("reading invocation counter file %s: %v", counterFile, err))
	}
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}
