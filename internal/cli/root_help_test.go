package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestMailSyncHelp_inheritsJSONFlag(t *testing.T) {
	opts := &rootOptions{}
	root := &cobra.Command{Use: "should-i-read", SilenceErrors: true}
	root.PersistentFlags().BoolVar(&opts.jsonOut, "json", false, "Print machine JSON instead of a human summary")
	root.AddCommand(newMailCmd(opts, t.TempDir()))

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"mail", "sync", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
}
	out := buf.String()
	if !strings.Contains(out, "--json") {
		t.Fatalf("mail sync --help missing --json:\n%s", out)
	}
}
