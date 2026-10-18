package cli

import (
	"strings"
	"testing"
)

func TestFormatConfigRedactedIncludesOTLP(t *testing.T) {
	t.Parallel()
	out := formatConfigRedacted(map[string]any{
		"polypus": map[string]string{
			"base_url":      "https://polypus.manticore-powan.ts.net",
			"otlp_endpoint": "polypus.manticore-powan.ts.net:4317",
		},
	})
	if !strings.Contains(out, "polypus.otlp_endpoint: polypus.manticore-powan.ts.net:4317") {
		t.Fatalf("missing otlp line:\n%s", out)
	}
}
