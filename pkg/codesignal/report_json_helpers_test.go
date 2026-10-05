package codesignal_test

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
	. "github.com/onsi/gomega"
)

// reportJSONTagNames returns the json tag names declared on t, in
// declaration order, recursing into embedded fields. Used to derive the
// expected key set directly from Report's own struct tags rather than a
// hand-maintained list, so a field added to Report is caught here without
// this test needing an update.
func reportJSONTagNames(t reflect.Type) []string {
	names := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			names = append(names, reportJSONTagNames(f.Type)...)
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		names = append(names, name)
	}
	return names
}

func rawReportKeys(report *codesignal.Report) []string {
	fields := rawReportFields(report)
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	return keys
}

func rawSignalKeys(raw json.RawMessage) []string {
	var m map[string]json.RawMessage
	Expect(json.Unmarshal(raw, &m)).To(Succeed())
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func rawReportFields(report *codesignal.Report) map[string]json.RawMessage {
	raw, err := json.Marshal(report)
	Expect(err).NotTo(HaveOccurred())
	var fields map[string]json.RawMessage
	Expect(json.Unmarshal(raw, &fields)).To(Succeed())
	return fields
}

func rawCoverageFields(fields map[string]json.RawMessage) map[string]json.RawMessage {
	Expect(fields).To(HaveKey("coverage"))
	var coverage map[string]json.RawMessage
	Expect(json.Unmarshal(fields["coverage"], &coverage)).To(Succeed())
	return coverage
}
