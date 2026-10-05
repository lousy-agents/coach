package jsbridge

import (
	"context"

	"testing"
)

func body_handlePart2Test_79(t *testing.T, tc struct {
	name string
	req  Request
	kind string
}) {
	resp := Handle(context.Background(), tc.req)
	if resp.Error == nil {
		t.Fatalf("no error, want kind %q", tc.kind)
	}
	if resp.Error.Kind != tc.kind {
		t.Fatalf("kind = %q (%s), want %q", resp.Error.Kind, resp.Error.Message, tc.kind)
	}
	if resp.Result != nil {
		t.Fatalf("Result = %+v, want nil for non-syntax errors", resp.Result)
	}
}
