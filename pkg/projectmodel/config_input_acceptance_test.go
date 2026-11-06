package projectmodel_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var _ = Describe("projectmodel.IsConfigInput", func() {
	DescribeTable("names the files a project model reads as configuration",
		func(name string, want bool) {
			Expect(projectmodel.IsConfigInput(name)).To(Equal(want))
		},
		Entry("a Go module manifest", "go.mod", true),
		Entry("a nested Go module manifest", "services/api/go.mod", true),
		Entry("a Go workspace file", "go.work", true),
		Entry("a package manifest", "package.json", true),
		Entry("a nested package manifest", "web/package.json", true),
		Entry("a tsconfig", "tsconfig.json", true),
		Entry("a tsconfig that other configs extend", "tsconfig.base.json", true),
		Entry("a nested tsconfig variant", "packages/ui/tsconfig.build.json", true),
		Entry("a config whose name only ends in tsconfig.json", "base-tsconfig.json", false),
		Entry("a lockfile", "package-lock.json", false),
		Entry("the Go checksum file", "go.sum", false),
		Entry("a backup of a manifest", "go.mod.bak", false),
		Entry("a Go source file", "pkg/db/db.go", false),
		Entry("a TypeScript source file", "pkg/db/d.ts", false),
	)
})
