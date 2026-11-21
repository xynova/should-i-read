package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestBareInvokePrintsAgentGuide(t *testing.T) {
	root := &cobra.Command{
		Use:           "should-i-read",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			agentOperatingGuide(cmd.OutOrStdout())
			return nil
		},
	}
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"ROLE",
		"AGENT OPERATING GUIDE",
		"COMMANDS BY RISK",
		"AUTOMATION RULES",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
