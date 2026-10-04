package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestWantJSON_env(t *testing.T) {
	t.Setenv(envJSONOut, "1")
	if !wantJSON(nil) {
		t.Fatal("expected env to force json")
	}
	t.Setenv(envJSONOut, "")
}

func TestWantJSON_persistentFlag(t *testing.T) {
	opts := &rootOptions{}
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().BoolVar(&opts.jsonOut, "json", false, "")
	sub := &cobra.Command{Use: "mail"}
	root.AddCommand(sub)
	opts.jsonOut = true
	if !wantJSON(sub) {
		t.Fatal("expected persistent --json")
	}
}
