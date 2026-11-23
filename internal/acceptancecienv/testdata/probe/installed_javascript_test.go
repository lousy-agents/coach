package probe

import (
	"flag"
	"os"
)

// The probe accepts Ginkgo's label filter, so a spec can run it the way a run
// that selects the ts-project-backend label is run.
var _ = flag.String("ginkgo.label-filter", "", "accepted and ignored")

// installedJS is where the run's repository holds the installed js/semantics,
// seen from this package's directory.
const installedJS = "../../../../js/semantics/node_modules"

// readInstalledJS reads each fixture file a spec may have installed, giving its
// content, or the error that kept it from being read.
func readInstalledJS() map[string]string {
	read := map[string]string{}
	for _, name := range []string{"fixture/index.js", "linked/file.txt"} {
		data, err := os.ReadFile(installedJS + "/" + name)
		if err != nil {
			read[name] = "error: " + err.Error()
			continue
		}
		read[name] = string(data)
	}
	return read
}
