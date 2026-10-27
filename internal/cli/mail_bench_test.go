package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/mailreport"
)

var errPimdirMissing = errors.New("pimdir.db not found")

func TestMailBenchHelp_listsVariantFlag(t *testing.T) {
	opts := &rootOptions{}
	root := &cobra.Command{Use: "should-i-read", SilenceErrors: true}
	root.AddCommand(newMailCmd(opts, t.TempDir()))

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"mail", "bench", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "--variant") {
		t.Fatalf("missing --variant:\n%s", out)
	}
	if !strings.Contains(out, "--chunks") {
		t.Fatalf("missing --chunks:\n%s", out)
	}
}

func TestMailBench_invalidVariantBeforePolypus(t *testing.T) {
	opts := &rootOptions{}
	repo := t.TempDir()
	root := &cobra.Command{Use: "should-i-read", SilenceErrors: true}
	root.AddCommand(newMailCmd(opts, repo))

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"mail", "bench", "--variant", "not-a-variant"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRequireItemBodies_okWhenInline(t *testing.T) {
	t.Parallel()
	err := requireItemBodies([]mailreport.Item{{ObjectHash: "a", Body: "text"}}, errPimdirMissing)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequireItemBodies_needsStoreWhenEmpty(t *testing.T) {
	t.Parallel()
	err := requireItemBodies([]mailreport.Item{{ObjectHash: "a"}}, errPimdirMissing)
	if err == nil {
		t.Fatal("expected error")
	}
}
