package evaldetect_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

const (
	apiDirect = "example.com/fx/pkg/api.Direct"
	storeSave = "example.com/fx/pkg/store.Save"
)

var _ = Describe("the Go layer-bypass detector", func() {
	When("every path from an upstream function to the downstream layer passes through the required layer", func() {
		It("reports nothing, ignoring test files and testdata", func() {
			root := bypassTree(bypassPolicy("usecases", false), nil)

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Bypasses).To(BeEmpty())
			Expect(report.Unresolved).To(BeZero())
		})
	})

	When("an upstream exported function calls the downstream layer directly through an allowed import", func() {
		It("reports exactly that path, at the first call, not via a forbidden import", func() {
			root := bypassTree(bypassPolicy("usecases", false), map[string]string{"pkg/api/direct.go": directBypass})

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Bypasses).To(ConsistOf(evaldetect.Bypass{
				Finding:            evaldetect.Finding{RuleID: bypassRule, Path: "pkg/api/direct.go", Subject: bypassRule + ":" + apiDirect + "->" + storeSave, Line: 5},
				Source:             apiDirect,
				Sink:               storeSave,
				Witness:            []string{apiDirect, storeSave},
				ViaForbiddenImport: false,
			}))
		})
	})

	When("the bypassing call crosses an import pair the policy forbids", func() {
		It("reports the path as via a forbidden import, so a layer violation already explains it", func() {
			root := bypassTree(bypassPolicy("usecases", true), map[string]string{"pkg/api/direct.go": directBypass})

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Bypasses).To(HaveLen(1))
			Expect(report.Bypasses[0].ViaForbiddenImport).To(BeTrue())
		})
	})

	DescribeTable("finds a bypass however the path is built",
		func(extra map[string]string, source string, witness []string) {
			root := bypassTree(bypassPolicy("usecases", false), extra)

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Bypasses).To(HaveLen(1))
			Expect(report.Bypasses[0].Source).To(Equal(source))
			Expect(report.Bypasses[0].Witness).To(Equal(witness))
		},
		Entry("through an unexported helper", map[string]string{
			"pkg/api/helper.go": "package api\n\nimport \"example.com/fx/pkg/store\"\n\nfunc Viahelper(s string) { save(s) }\n\nfunc save(s string) { store.Save(s) }\n",
		}, "example.com/fx/pkg/api.Viahelper", []string{"example.com/fx/pkg/api.Viahelper", "example.com/fx/pkg/api.save", storeSave}),
		Entry("through a package outside every layer", map[string]string{
			"pkg/api/tool.go":      "package api\n\nimport \"example.com/fx/tools/relay\"\n\nfunc Viatool(s string) { relay.Pass(s) }\n",
			"tools/relay/relay.go": "package relay\n\nimport \"example.com/fx/pkg/store\"\n\nfunc Pass(s string) { store.Save(s) }\n",
		}, "example.com/fx/pkg/api.Viatool", []string{"example.com/fx/pkg/api.Viatool", "example.com/fx/tools/relay.Pass", storeSave}),
		Entry("through a method on a concrete type", map[string]string{
			"pkg/api/method.go": "package api\n\nimport \"example.com/fx/pkg/store\"\n\ntype Server struct{}\n\nfunc (Server) Serve(s string) { store.Save(s) }\n",
		}, "(example.com/fx/pkg/api.Server).Serve", []string{"(example.com/fx/pkg/api.Server).Serve", storeSave}),
		Entry("from a nested module in the upstream layer", map[string]string{
			"pkg/api/nested/go.mod":    "module example.com/nested\n\ngo 1.26.0\n",
			"pkg/api/nested/nested.go": "package nested\n\nimport \"example.com/fx/pkg/store\"\n\nfunc Edge(s string) { store.Save(s) }\n",
		}, "example.com/nested.Edge", []string{"example.com/nested.Edge", storeSave}),
	)

	DescribeTable("follows a call to a generic function or method to its declaration",
		func(extra map[string]string, source string, witness []string) {
			root := bypassTree(bypassPolicy("usecases", false), extra)

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Bypasses).To(HaveLen(1))
			Expect(report.Bypasses[0].Source).To(Equal(source))
			Expect(report.Bypasses[0].Witness).To(Equal(witness))
			Expect(report.Unresolved).To(BeZero())
		},
		Entry("a method of an instantiated generic type", map[string]string{
			"pkg/api/box.go": "package api\n\nimport \"example.com/fx/pkg/store\"\n\ntype box[T any] struct{ v T }\n\nfunc (b *box[T]) get() T { store.Save(\"x\"); return b.v }\n\nfunc Viabox() int { b := &box[int]{}; return b.get() }\n",
		}, "example.com/fx/pkg/api.Viabox", []string{"example.com/fx/pkg/api.Viabox", "(*example.com/fx/pkg/api.box[T]).get", storeSave}),
		Entry("an explicitly instantiated generic function", map[string]string{
			"pkg/api/via.go": "package api\n\nimport \"example.com/fx/pkg/store\"\n\nfunc via[T any](t T) { store.Save(\"x\") }\n\nfunc Viageneric() { via[int](1) }\n",
		}, "example.com/fx/pkg/api.Viageneric", []string{"example.com/fx/pkg/api.Viageneric", "example.com/fx/pkg/api.via", storeSave}),
	)

	When("two calls on one line both reach the downstream layer", func() {
		It("reports the witness through the call that starts first on the line", func() {
			root := bypassTree(bypassPolicy("usecases", false), map[string]string{
				"pkg/api/tieline.go": "package api\n\nimport \"example.com/fx/pkg/store\"\n\nfunc Tieline(s string) { zed(s); abe(s) }\n\nfunc abe(s string) { store.Save(s) }\n\nfunc zed(s string) { store.Save(s) }\n",
			})

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Bypasses).To(HaveLen(1))
			Expect(report.Bypasses[0].Witness).To(Equal([]string{"example.com/fx/pkg/api.Tieline", "example.com/fx/pkg/api.zed", storeSave}))
		})
	})

	When("the sink itself calls further into the downstream layer", func() {
		It("reports only the first downstream function reached", func() {
			root := bypassTree(bypassPolicy("usecases", false), map[string]string{
				"pkg/api/direct.go":  directBypass,
				"pkg/store/store.go": "package store\n\ntype Record struct{ Name string }\n\nfunc Save(s string) { Write(s) }\n\nfunc Write(s string) {}\n",
			})

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Bypasses).To(HaveLen(1))
			Expect(report.Bypasses[0].Sink).To(Equal(storeSave))
		})
	})

	When("a layer is neither upstream nor downstream of the required layer", func() {
		It("passes through it, and never starts or ends a bypass in it", func() {
			root := bypassTree(bypassPolicy("usecases", false), map[string]string{
				"pkg/api/viashared.go": "package api\n\nimport \"example.com/fx/pkg/shared\"\n\nfunc Viashared(s string) { shared.Relay(s) }\n",
				"pkg/shared/shared.go": "package shared\n\nimport \"example.com/fx/pkg/store\"\n\nfunc Relay(s string) { store.Save(s) }\n",
			})

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Bypasses).To(HaveLen(1))
			Expect(report.Bypasses[0].Source).To(Equal("example.com/fx/pkg/api.Viashared"))
			Expect(report.Bypasses[0].Witness).To(Equal([]string{"example.com/fx/pkg/api.Viashared", "example.com/fx/pkg/shared.Relay", storeSave}))
		})
	})

	When("a method on an unexported type in the upstream layer calls the store", func() {
		It("is not a source, because nothing outside its package can call it by name", func() {
			root := bypassTree(bypassPolicy("usecases", false), map[string]string{
				"pkg/api/hidden.go": "package api\n\nimport \"example.com/fx/pkg/store\"\n\ntype hidden struct{}\n\nfunc (hidden) Serve(s string) { store.Save(s) }\n",
			})

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Bypasses).To(BeEmpty())
		})
	})

	DescribeTable("a look-alike that never calls past the required layer is not reported",
		func(extra map[string]string) {
			root := bypassTree(bypassPolicy("usecases", false), extra)

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Bypasses).To(BeEmpty())
		},
		Entry("a downstream type used only as a value", map[string]string{
			"pkg/api/value.go": "package api\n\nimport \"example.com/fx/pkg/store\"\n\nfunc Value() store.Record { return store.Record{Name: \"v\"} }\n",
		}),
		Entry("an unexported upstream function that is never called", map[string]string{
			"pkg/api/private.go": "package api\n\nimport \"example.com/fx/pkg/store\"\n\nfunc private(s string) { store.Save(s) }\n",
		}),
		Entry("a call into the required layer that then reaches the store", map[string]string{
			"pkg/api/again.go": "package api\n\nimport \"example.com/fx/pkg/usecases\"\n\nfunc Again(s string) { usecases.Register(s) }\n",
		}),
	)

	When("a call cannot be resolved statically", func() {
		It("counts it as unresolved coverage and reports no path through it", func() {
			root := bypassTree(bypassPolicy("usecases", false), map[string]string{
				"pkg/api/dynamic.go": "package api\n\nimport \"example.com/fx/pkg/store\"\n\ntype saver interface{ Save(string) }\n\ntype impl struct{}\n\nfunc (impl) Save(s string) { store.Save(s) }\n\nfunc Viainterface(s string) { var v saver = impl{}; v.Save(s) }\n\nfunc Viavalue(s string) { f := store.Save; f(s) }\n",
			})

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Unresolved).To(Equal(2))
			for _, bypass := range report.Bypasses {
				Expect(bypass.Source).NotTo(BeElementOf("example.com/fx/pkg/api.Viainterface", "example.com/fx/pkg/api.Viavalue"))
			}
		})
	})

	DescribeTable("counts a call no static resolution follows as unresolved, without failing",
		func(file string) {
			root := bypassTree(bypassPolicy("usecases", false), map[string]string{"pkg/api/unresolved.go": file})

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Unresolved).To(Equal(1))
			Expect(report.Bypasses).To(BeEmpty())
		},
		Entry("a call through a struct field that holds a function",
			"package api\n\nimport \"example.com/fx/pkg/store\"\n\ntype holder struct{ save func(string) }\n\nfunc Viafield(s string) { h := holder{save: store.Save}; h.save(s) }\n"),
		Entry("a call to a local function declared without a body",
			"package api\n\nfunc external(s string)\n\nfunc Viabodyless(s string) { external(s) }\n"),
	)

	When("two shortest paths are equally long", func() {
		It("reports the witness whose first differing call comes earlier in the file", func() {
			root := bypassTree(bypassPolicy("usecases", false), map[string]string{
				"pkg/api/tie.go": "package api\n\nimport \"example.com/fx/pkg/store\"\n\nfunc Tie(s string) {\n\tsecond(s)\n\tfirst(s)\n}\n\nfunc first(s string) { store.Save(s) }\n\nfunc second(s string) { store.Save(s) }\n",
			})

			for range 3 {
				report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
				Expect(err).NotTo(HaveOccurred())
				Expect(report.Bypasses).To(HaveLen(1))
				Expect(report.Bypasses[0].Witness).To(Equal([]string{"example.com/fx/pkg/api.Tie", "example.com/fx/pkg/api.second", storeSave}))
				Expect(report.Bypasses[0].Line).To(Equal(6))
			}
		})
	})

	When("the policy names no required layer", func() {
		It("reports nothing and says why", func() {
			root := bypassTree(bypassPolicy("", false), map[string]string{"pkg/api/direct.go": directBypass})

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Bypasses).To(BeEmpty())
			Expect(report.Reason).To(Equal("the policy names no required_layer"))
		})
	})

	When("a package does not type-check", func() {
		It("refuses the program, naming the package, rather than grading it in part", func() {
			root := bypassTree(bypassPolicy("usecases", false), map[string]string{
				"pkg/api/bad.go": "package api\n\nfunc Bad() int { return \"no\" }\n",
			})

			_, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).To(MatchError(evaldetect.ErrRefused))
			Expect(err).To(MatchError(ContainSubstring("example.com/fx/pkg/api")))
		})
	})

	When("the call graph has exactly as many functions as the stated limit", func() {
		It("searches the program", func() {
			restore := evaldetect.SetMaxCallGraphNodes(4)
			defer restore()
			root := bypassTree(bypassPolicy("usecases", false), map[string]string{"pkg/api/direct.go": directBypass})

			report, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).NotTo(HaveOccurred())
			Expect(report.Bypasses).To(HaveLen(1))
		})
	})

	When("two local packages import each other", func() {
		It("refuses the program, naming the cycle, rather than recursing without end", func() {
			root := bypassTree(bypassPolicy("usecases", false), map[string]string{
				"pkg/shared/ping/ping.go": "package ping\n\nimport \"example.com/fx/pkg/shared/pong\"\n\nfunc Ping() { pong.Pong() }\n",
				"pkg/shared/pong/pong.go": "package pong\n\nimport \"example.com/fx/pkg/shared/ping\"\n\nfunc Pong() { ping.Ping() }\n",
			})

			_, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).To(MatchError(evaldetect.ErrRefused))
			Expect(err).To(MatchError(ContainSubstring("import cycle")))
			Expect(err).To(MatchError(ContainSubstring("example.com/fx/pkg/shared/ping")))
			Expect(err).To(MatchError(ContainSubstring("example.com/fx/pkg/shared/pong")))
		})
	})

	When("the call graph has more functions than the stated limit", func() {
		It("refuses the program, naming the limit, rather than truncating the search", func() {
			restore := evaldetect.SetMaxCallGraphNodes(3)
			defer restore()
			root := bypassTree(bypassPolicy("usecases", false), map[string]string{"pkg/api/direct.go": directBypass})

			_, err := evaldetect.GoLayerBypass(root, loadPolicy(root))
			Expect(err).To(MatchError(evaldetect.ErrRefused))
			Expect(err).To(MatchError(ContainSubstring("more than 3 functions")))
		})
	})
})

var _ = Describe("comparing the bypass detector with Coach", func() {
	It("records a bypass only one side reports as a disagreement, and grades neither as a defect", func() {
		detector := []evaldetect.Bypass{
			{Finding: evaldetect.Finding{RuleID: bypassRule, Path: "pkg/api/direct.go", Subject: "x", Line: 5}, Source: apiDirect, Sink: storeSave},
			{Finding: evaldetect.Finding{RuleID: bypassRule, Path: "pkg/api/both.go", Subject: "y", Line: 3}, Source: "example.com/fx/pkg/api.Both", Sink: storeSave},
		}
		coach := []evaldetect.Finding{
			{RuleID: bypassRule, Path: "pkg/api/both.go", Line: 3},
			{RuleID: bypassRule, Path: "pkg/api/coachonly.go", Line: 9},
		}

		comparison := evaldetect.CompareBypasses(detector, coach)

		Expect(comparison.Agreed).To(Equal([]string{"pkg/api/both.go"}))
		Expect(comparison.Disagreements).To(ConsistOf(
			evaldetect.BypassDisagreement{Path: "pkg/api/direct.go", Side: "detector_only", Source: apiDirect, Sink: storeSave},
			evaldetect.BypassDisagreement{Path: "pkg/api/coachonly.go", Side: "coach_only"},
		))
		Expect(comparison.Graded).To(BeFalse(), "a disagreement is recorded, never graded as a defect")
	})
})
