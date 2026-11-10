package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestMailCartApplyHelp(t *testing.T) {
	t.Parallel()
	opts := &rootOptions{}
	root := &cobra.Command{Use: "should-i-read", SilenceErrors: true}
	root.PersistentFlags().BoolVar(&opts.jsonOut, "json", false, "")
	root.AddCommand(newMailCmd(opts, t.TempDir()))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"mail", "cart", "apply", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "--confirm") {
		t.Fatal("missing --confirm in help")
	}
}

func TestMailCartApplyWithoutConfirm(t *testing.T) {
	t.Parallel()
	opts := &rootOptions{}
	root := &cobra.Command{Use: "should-i-read", SilenceErrors: true}
	root.AddCommand(newMailCmd(opts, t.TempDir()))
	root.SetArgs([]string{"mail", "cart", "apply", "jobid"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}
