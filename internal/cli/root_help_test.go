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
	if !strings.Contains(out, "--exclude-collection") {
		t.Fatalf("mail sync --help missing --exclude-collection:\n%s", out)
	}
}

func TestMailSimilarHelp(t *testing.T) {
	opts := &rootOptions{}
	root := &cobra.Command{Use: "should-i-read", SilenceErrors: true}
	root.PersistentFlags().BoolVar(&opts.jsonOut, "json", false, "Print machine JSON instead of a human summary")
	root.AddCommand(newMailCmd(opts, t.TempDir()))
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"mail", "similar", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "--k") {
		t.Fatalf("mail similar --help missing --k:\n%s", out)
	}
	if !strings.Contains(out, "--json") {
		t.Fatalf("mail similar --help missing --json:\n%s", out)
	}
}
