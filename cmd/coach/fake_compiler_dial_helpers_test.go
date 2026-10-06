package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func writeFakeCompilerDialModule(host string, port int) string {
	dir, err := os.MkdirTemp("", "coach-d2-fake-compiler-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)
	dial := fmt.Sprintf("import net from 'node:net';\ntry {\n  await new Promise((resolve, reject) => {\n    const socket = net.connect({host:%q, port:%d}, () => { socket.end(); resolve(); });\n    socket.on('error', reject);\n  });\n} catch (err) {\n  console.error(err.code || err.message);\n  process.exitCode = 1;\n}\n", host, port)
	Expect(os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"fake-ts-compiler","type":"module","main":"dial.js"}`+"\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(dir, "dial.js"), []byte(dial), 0o644)).To(Succeed())
	return dir
}

func fakeCompilerLoader() string {
	dir, err := os.MkdirTemp("", "coach-d2-fake-loader-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)
	src := "import { pathToFileURL } from 'node:url';\nconst flag = process.argv.find((a)=>a.startsWith('--compiler-module='));\nif (!flag) { process.stderr.write('missing --compiler-module=\\n'); process.exit(2); }\nawait import(pathToFileURL(flag.slice('--compiler-module='.length)+'/dial.js').href);\n"
	path := filepath.Join(dir, "load.mjs")
	Expect(os.WriteFile(path, []byte(src), 0o644)).To(Succeed())
	return path
}

func runFakeCompilerDial(prefix []string, moduleDir string) (stdout, stderr string, err error) {
	loader := fakeCompilerLoader()
	args := append(append([]string{}, prefix...), probedNodeExecPath(), loader, "--compiler-module="+moduleDir)
	cmd := exec.Command(args[0], args[1:]...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}
