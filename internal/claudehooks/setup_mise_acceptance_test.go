package claudehooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// hookEnv is a throwaway filesystem sandbox for driving setup-mise.sh: a fake
// HOME, a project directory holding mise.toml, and a bin directory whose fake
// mise/npm the hook resolves through PATH.
type hookEnv struct {
	tmp, home, project, bin, localBin, envFile string
}

func newHookEnv(miseToml string) hookEnv {
	t := GinkgoT()
	tmp := t.TempDir()
	e := hookEnv{
		tmp:      tmp,
		home:     filepath.Join(tmp, "home"),
		project:  filepath.Join(tmp, "project"),
		bin:      filepath.Join(tmp, "bin"),
		localBin: filepath.Join(tmp, "home", ".local", "bin"),
		envFile:  filepath.Join(tmp, "env"),
	}
	for _, d := range []string{e.home, e.project, e.bin, e.localBin} {
		Expect(os.MkdirAll(d, 0755)).To(Succeed())
	}
	if miseToml != "" {
		Expect(os.WriteFile(filepath.Join(e.project, "mise.toml"), []byte(miseToml), 0644)).To(Succeed())
	}
	// A no-op npm keeps the "needs install" path from reaching the network.
	e.writeBin("npm", "#!/bin/sh\nexit 0\n")
	return e
}

func (e hookEnv) writeBin(name, script string) {
	Expect(os.WriteFile(filepath.Join(e.bin, name), []byte(script), 0755)).To(Succeed())
}

// run executes the real hook with only the sandbox on PATH. projectDir is what
// CLAUDE_PROJECT_DIR is set to; pass "" to leave it unset.
func (e hookEnv) run(projectDir string) (stdout, stderr string, err error) {
	scriptPath, absErr := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "setup-mise.sh"))
	Expect(absErr).NotTo(HaveOccurred())

	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = e.project
	cmd.Env = []string{
		"HOME=" + e.home,
		"CLAUDE_CODE_REMOTE=true",
		"CLAUDE_ENV_FILE=" + e.envFile,
		"PATH=" + e.bin + ":/usr/bin:/bin",
	}
	if projectDir != "" {
		cmd.Env = append(cmd.Env, "CLAUDE_PROJECT_DIR="+projectDir)
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func (e hookEnv) envFileContents() string {
	data, err := os.ReadFile(e.envFile)
	Expect(err).NotTo(HaveOccurred(), "expected CLAUDE_ENV_FILE to be written")
	return string(data)
}

const pinnedMiseToml = `min_version = "2026.7.7"
[tools]
go = "1.26.5"
node = "24"
`

// recordingMise builds a fake mise that appends every invocation to logPath and
// fails whichever subcommands the caller names.
func recordingMise(logPath, version, binPaths string, failing map[string]bool) string {
	exitFor := func(sub string) string {
		if failing[sub] {
			return "1"
		}
		return "0"
	}
	return `#!/bin/sh
echo "$*" >> ` + logPath + `
if [ "$1" = "--version" ]; then echo "` + version + `"; exit 0; fi
if [ "$1" = "trust" ]; then exit ` + exitFor("trust") + `; fi
if [ "$1" = "install" ]; then
  if [ "$#" -eq 1 ]; then exit ` + exitFor("install") + `; fi
  exit ` + exitFor("install-tools") + `
fi
if [ "$1" = "bin-paths" ]; then echo "` + binPaths + `"; exit ` + exitFor("bin-paths") + `; fi
exit 0
`
}

var _ = Describe("setup-mise.sh SessionStart hook", func() {
	Describe("persisting the toolchain PATH", func() {
		When("every mise install attempt fails", func() {
			It("still writes CLAUDE_ENV_FILE so the session keeps a usable PATH", func() {
				e := newHookEnv(pinnedMiseToml)
				log := filepath.Join(e.tmp, "mise-log")
				e.writeBin("mise", recordingMise(log, "2026.7.7", e.localBin,
					map[string]bool{"install": true, "install-tools": true}))

				_, stderr, err := e.run(e.project)

				Expect(err).NotTo(HaveOccurred(), "hook must not abort when installs fail; stderr: %s", stderr)
				Expect(e.envFileContents()).To(ContainSubstring(filepath.Join(e.home, ".local", "bin")))
			})
		})

		When("mise bin-paths fails after the tools are installed", func() {
			It("still writes CLAUDE_ENV_FILE instead of aborting at the last step", func() {
				e := newHookEnv(pinnedMiseToml)
				log := filepath.Join(e.tmp, "mise-log")
				e.writeBin("mise", recordingMise(log, "2026.7.7", e.localBin,
					map[string]bool{"bin-paths": true}))

				_, stderr, err := e.run(e.project)

				Expect(err).NotTo(HaveOccurred(), "hook must not abort on bin-paths failure; stderr: %s", stderr)
				Expect(e.envFileContents()).To(ContainSubstring(filepath.Join(e.home, ".local", "bin")))
			})
		})

		When("the hook runs repeatedly against a CLAUDE_ENV_FILE that persists", func() {
			It("appends the PATH export only once", func() {
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
			})
		})
	})

	Describe("deciding whether mise itself needs an upgrade", func() {
		When("min_version carries a zero-padded month newer than the installed mise", func() {
			It("treats the installed mise as stale and installs the pinned version", func() {
				e := newHookEnv(`min_version = "2026.09.0"
[tools]
go = "1.26.5"
node = "24"
`)
				npmLog := filepath.Join(e.tmp, "npm-log")
				e.writeBin("npm", "#!/bin/sh\necho \"$*\" >> "+npmLog+"\nexit 0\n")
				log := filepath.Join(e.tmp, "mise-log")
				e.writeBin("mise", recordingMise(log, "2026.8.0", e.localBin, nil))

				_, stderr, err := e.run(e.project)

				Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr)
				data, readErr := os.ReadFile(npmLog)
				Expect(readErr).NotTo(HaveOccurred(), "expected npm install to run for a stale mise")
				Expect(string(data)).To(ContainSubstring("mise@2026.09.0"))
			})
		})
	})

	Describe("locating the project configuration", func() {
		When("the resolved project directory has no mise.toml", func() {
			It("fails with a message naming the directory it searched", func() {
				e := newHookEnv("")
				log := filepath.Join(e.tmp, "mise-log")
				e.writeBin("mise", recordingMise(log, "2026.7.7", e.localBin, nil))

				_, stderr, err := e.run(e.project)

				Expect(err).To(HaveOccurred(), "a missing mise.toml must not be silently ignored")
				Expect(stderr).To(ContainSubstring("mise.toml not found"))
				Expect(stderr).To(ContainSubstring(e.project))
			})
		})

		When("run against the repository's own committed mise.toml", func() {
			It("extracts a usable min_version", func() {
				repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
				Expect(err).NotTo(HaveOccurred())

				e := newHookEnv("")
				log := filepath.Join(e.tmp, "mise-log")
				e.writeBin("mise", recordingMise(log, "9999.1.0", e.localBin, nil))

				_, stderr, runErr := e.run(repoRoot)

				Expect(runErr).NotTo(HaveOccurred(),
					"the hook must parse the real mise.toml; stderr: %s", stderr)
				Expect(stderr).NotTo(ContainSubstring("min_version"),
					"real mise.toml must satisfy the hook's min_version format check")
			})
		})
	})
})
