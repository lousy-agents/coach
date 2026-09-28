package codesignal_test

import (
	"encoding/json"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_reportShapeAcceptanceTest_keepsSchemaVersion2SMarshalledKeySetInSyncWithRe_114(expectedKeys []string, schema2OptionalKeys []string) {
	expectedSchema2Keys := make([]string, 0, len(expectedKeys))
	for _, name := range expectedKeys {
		if schemaKeyDropped(name, schema2OptionalKeys) {
			continue
		}
		expectedSchema2Keys = append(expectedSchema2Keys, name)
	}

	schema2 := codesignal.Report{SchemaVersion: "2"}
	Expect(rawReportKeys(&schema2)).To(ConsistOf(expectedSchema2Keys))
}

func body_reportShapeAcceptanceTest_keepsSchemaVersion1SMarshalledKeySetInSyncWithRe_133(expectedKeys []string, schema1ExcludedKeys []string) {
	expectedSchema1Keys := make([]string, 0, len(expectedKeys))
	for _, name := range expectedKeys {
		if schemaKeyDropped(name, schema1ExcludedKeys) {
			continue
		}
		expectedSchema1Keys = append(expectedSchema1Keys, name)
	}

	schema1 := codesignal.Report{SchemaVersion: "1"}
	Expect(rawReportKeys(&schema1)).To(ConsistOf(expectedSchema1Keys))
}

func schemaKeyDropped(name string, dropped []string) bool {
	for _, p := range dropped {
		if name == p {
			return true
		}
	}
	return false
}

func body_reportShapeAcceptanceTest_210(rawSignals []json.RawMessage) map[string]json.RawMessage {
	for _, r := range rawSignals {
		var m map[string]json.RawMessage
		Expect(json.Unmarshal(r, &m)).To(Succeed())
		var subject string
		if raw, ok := m["subject"]; ok {
			Expect(json.Unmarshal(raw, &subject)).To(Succeed())
		}
		if subject == "Update" {
			return m
		}
	}
	return nil
}
