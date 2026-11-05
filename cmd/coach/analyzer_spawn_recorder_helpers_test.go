package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/gomega"
)

// analyzerArgMarker is the argument every analyzer launch carries and no
// version probe does, which is how the recorder tells them apart.
const analyzerArgMarker = "--compiler-module="

// analyzerSpawn.Environ is the environment as /bin/sh exports it, which is a
// superset of what the launcher passed: bash adds PWD, SHLVL and _. dash adds
// PWD and drops names that are not shell identifiers, so leak assertions must
// use identifier names and never assert an exact environment.
type analyzerSpawn struct {
	PID     int
	Environ string
}

// analyzerSpawnRecorder stands in for the host `node`. The launcher resolves
// node with `<node on PATH> -p process.execPath` and spawns the path that
// prints, never the PATH entry itself, so the recorder answers that probe with
// its own path and passes every other call through to the real node. It writes
// its record before the analyzer runs, so a launch cannot be missed however
// briefly the analyzer lives.
type analyzerSpawnRecorder struct {
	nodePath string
	spawnDir string
}

// newAnalyzerSpawnRecorder keeps the records outside dir/bin because the
// launcher gives the analyzer dir/bin as its whole PATH.
func newAnalyzerSpawnRecorder(dir, execTarget string) (analyzerSpawnRecorder, error) {
	binDir := filepath.Join(dir, "bin")
	recorder := analyzerSpawnRecorder{
		nodePath: filepath.Join(binDir, "node"),
		spawnDir: filepath.Join(dir, "spawns"),
	}
	for _, made := range []string{binDir, recorder.spawnDir} {
		if err := os.MkdirAll(made, 0o755); err != nil {
			return analyzerSpawnRecorder{}, err
		}
	}
	script := recorderScript(recorder.nodePath, recorder.spawnDir, execTarget)
	if err := os.WriteFile(recorder.nodePath, []byte(script), 0o755); err != nil {
		return analyzerSpawnRecorder{}, err
	}
	return recorder, nil
}

// recorderScriptTemplate uses only shell builtins plus /usr/bin/env,
// /usr/bin/mktemp and /bin/mv, by absolute path, because the launcher gives
// the analyzer a PATH holding nothing but the recorder's own directory. A
// record is the pid line followed by `env`'s output as /bin/sh exports it (see
// analyzerSpawn). mktemp makes each record's name unique by construction, and
// the record is renamed to *.rec only once it is complete, so a reader never
// sees a partial one. A launch that cannot be recorded exits 97 with a message
// instead of running unrecorded.
const recorderScriptTemplate = `#!/bin/sh
if [ "$#" -eq 2 ] && [ "$1" = "-p" ] && [ "$2" = "process.execPath" ]; then
	printf '%s\n' @SELF@
	exit 0
fi
case "$*" in
*@MARKER@*)
	record="$$
$(/usr/bin/env)"
	tmp=$(/usr/bin/mktemp @SPAWNS@/.rec.XXXXXX)
	if [ -z "$tmp" ] || ! printf '%s\n' "$record" > "$tmp" || ! /bin/mv "$tmp" "$tmp.rec"; then
		printf 'analyzer spawn recorder: could not record launch in %s\n' @SPAWNS@ >&2
		exit 97
	fi
	;;
esac
exec @TARGET@ "$@"
`

func recorderScript(self, spawnDir, execTarget string) string {
	return strings.NewReplacer(
		"@SELF@", shellQuote(self),
		"@MARKER@", analyzerArgMarker,
		"@SPAWNS@", shellQuote(spawnDir),
		"@TARGET@", shellQuote(execTarget),
	).Replace(recorderScriptTemplate)
}

// takeSpawns consumes the finished records (*.rec). A record that cannot be
// parsed or removed is reported in the returned error and stays on disk, so a
// launch is never dropped silently or counted twice.
func (r analyzerSpawnRecorder) takeSpawns() ([]analyzerSpawn, error) {
	entries, err := os.ReadDir(r.spawnDir)
	if err != nil {
		return nil, err
	}
	var spawns []analyzerSpawn
	var problems []error
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".rec") {
			continue
		}
		spawn, err := r.takeRecord(entry.Name())
		if err != nil {
			problems = append(problems, err)
			continue
		}
		spawns = append(spawns, spawn)
	}
	return spawns, errors.Join(problems...)
}

func (r analyzerSpawnRecorder) mustTakeSpawns(g Gomega) []analyzerSpawn {
	spawns, err := r.takeSpawns()
	g.Expect(err).NotTo(HaveOccurred())
	return spawns
}

func (r analyzerSpawnRecorder) takeRecord(name string) (analyzerSpawn, error) {
	path := filepath.Join(r.spawnDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return analyzerSpawn{}, err
	}
	spawn, ok := parseSpawnRecord(string(data))
	if !ok {
		return analyzerSpawn{}, fmt.Errorf("record %s is not a pid line followed by an environment", name)
	}
	return spawn, os.Remove(path)
}

func parseSpawnRecord(record string) (analyzerSpawn, bool) {
	pidLine, environ, _ := strings.Cut(record, "\n")
	pid, err := strconv.Atoi(strings.TrimSpace(pidLine))
	if err != nil {
		return analyzerSpawn{}, false
	}
	return analyzerSpawn{PID: pid, Environ: environ}, true
}
