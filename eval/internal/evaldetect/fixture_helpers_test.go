package evaldetect_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

// writeTree writes each path → content pair under a fresh temporary root.
func writeTree(root string, files map[string]string) string {
	for path, content := range files {
		full := filepath.Join(root, path)
		Expect(os.MkdirAll(filepath.Dir(full), 0o755)).To(Succeed())
		Expect(os.WriteFile(full, []byte(content), 0o644)).To(Succeed())
	}
	return root
}

const goLayerPolicy = `{"schema_version":"1","roots":["."],"layers":[{"name":"domain","prefixes":["pkg/domain"]},{"name":"app","prefixes":["pkg/app"]}],"forbidden_imports":[{"from":"domain","to":"app"}]}`

const tsLayerPolicy = `{"schema_version":"1","roots":["."],"layers":[{"name":"domain","prefixes":["src/domain"]},{"name":"app","prefixes":["src/app"]}],"forbidden_imports":[{"from":"domain","to":"app"}]}`

// typescriptDir is the pinned compiler that js/semantics installs, which the
// TS layer graph must use rather than any compiler on the host.
func typescriptDir() string {
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "js", "semantics", "node_modules", "typescript"))
	Expect(err).NotTo(HaveOccurred())
	_, err = os.Stat(filepath.Join(dir, "package.json"))
	Expect(err).NotTo(HaveOccurred(), "run npm ci --ignore-scripts in js/semantics first")
	return dir
}

// gocognitBinary is the gocognit the eval module pins with its tool directive.
func gocognitBinary() string {
	command := exec.Command("go", "tool", "-n", "gocognit")
	command.Dir = filepath.Join("..", "..")
	output, err := command.Output()
	Expect(err).NotTo(HaveOccurred(), "go tool -n gocognit must resolve from the eval module")
	return strings.TrimSpace(string(output))
}

// ifChain returns a Go function whose cognitive complexity is exactly n: n
// sequential if statements, each +1 with no nesting.
func ifChain(name string, n int) string {
	var body strings.Builder
	for i := 0; i < n; i++ {
		body.WriteString("\tif x == " + itoa(i) + " {\n\t\ty++\n\t}\n")
	}
	return "func " + name + "(x int) int {\n\ty := 0\n" + body.String() + "\treturn y\n}\n"
}

func itoa(i int) string {
	digits := "0123456789"
	if i < 10 {
		return string(digits[i])
	}
	return itoa(i/10) + string(digits[i%10])
}
