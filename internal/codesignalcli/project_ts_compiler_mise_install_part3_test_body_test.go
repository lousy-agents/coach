package codesignalcli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func body_projectTsCompilerMiseInstallPart3Test_85(t *testing.T, readiness *ReadinessResult) {
	result := RunPrepareCompilerMiseSetup(context.Background(), t.TempDir(), "HEAD", "", readiness, strings.NewReader(""), &bytes.Buffer{})
	if !result.NoChoicesOffered {
		t.Fatalf("NoChoicesOffered = false, want true: %+v", result)
	}
	if result.Cancelled || result.Attempted || result.Trusted || result.Succeeded {
		t.Fatalf("expected every other outcome field to stay zero, got %+v", result)
	}
}
