package codesignalcli

import (
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// AC-VER-5/AC-VER-6 (#334, epic #280): reachability facts describe HEAD
// only. A backend result field such as BaseFacts or ResolvedFacts would let
// base-side or lifecycle-classified facts into the report pipeline.
var _ = Describe("ProjectBackendResult reachability facts", func() {
	It("carries facts on exactly one head-only field named Facts", func() {
		resultType := reflect.TypeOf(ProjectBackendResult{})
		var factFields []string
		for i := 0; i < resultType.NumField(); i++ {
			if name := resultType.Field(i).Name; strings.Contains(name, "Fact") {
				factFields = append(factFields, name)
			}
		}
		Expect(factFields).To(Equal([]string{"Facts"}))
	})
})
